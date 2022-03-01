package gintrace

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
)

// ctxKey is used for context value
type ctxKey int

// CtxKeyFullStack indicates if should store full stack frames
// CtxKeyTid is inserted trace id on the beginning of the request
const (
	ctxKeyBadaCtx ctxKey = iota

	XCurrTid  = "X-Curr-Tid"
	XPrevTid  = "X-Prev-Tid"
	XEntryTid = "X-Entry-Tid"
)

var (
	ErrInvalidBadaCtx = errors.New("invalid BadaCtx")
)

// BadaCtx is inserted to the request context, on the beginning of the request
type BadaCtx struct {
	PrevTid  string
	CurrTid  string
	EntryTid string
}

func MustGetBadaCtx(ctx context.Context) *BadaCtx {
	badaCtx, ok := GetBadaCtx(ctx)
	if !ok {
		panic("bada context should be inserted into the context, find out why you pass a ctx without it")
	}
	return badaCtx
}

func GetBadaCtx(ctx context.Context) (badaCtx *BadaCtx, ok bool) {
	badaCtx, ok = ctx.Value(ctxKeyBadaCtx).(*BadaCtx)
	ok = ok && badaCtx.IsValid()
	return
}

func (b *BadaCtx) IsValid() bool {
	// both entry and curr tid must exist
	return b != nil && b.CurrTid != "" && b.EntryTid != ""
}

func (b *BadaCtx) SetHeader(h http.Header) {
	if b.PrevTid != "" {
		h.Set(XPrevTid, b.PrevTid)
	}

	if b.EntryTid == "" {
		h.Set(XEntryTid, b.CurrTid)
	} else {
		h.Set(XEntryTid, b.EntryTid)
	}

	h.Set(XCurrTid, b.CurrTid)
}

func (b *BadaCtx) EmbedIntoContext(ctx context.Context) (context.Context, error) {
	if !b.IsValid() {
		return nil, ErrInvalidBadaCtx
	}
	return context.WithValue(ctx, ctxKeyBadaCtx, b), nil
}

func ExtractBadaCtxFromHeader(h http.Header) (b BadaCtx) {
	b.CurrTid = h.Get(XCurrTid)
	b.PrevTid = h.Get(XPrevTid)
	b.EntryTid = h.Get(XEntryTid)
	return
}

func NewBadaCtx() BadaCtx {
	tid := primitive.NewObjectID().Hex()
	return BadaCtx{
		EntryTid: tid,
		CurrTid:  tid,
	}
}

func ChainBadaCtx(prev *BadaCtx) (curr BadaCtx, err error) {
	if !prev.IsValid() {
		return curr, ErrInvalidBadaCtx
	}
	curr.EntryTid = prev.EntryTid
	curr.PrevTid = prev.CurrTid
	curr.CurrTid = primitive.NewObjectID().Hex()
	return
}

func WithBidTrace(c *gin.Context) {
	ctx := c.Request.Context()
	b := ExtractBadaCtxFromHeader(c.Request.Header)
	var err error
	if b.EntryTid == "" {
		b = NewBadaCtx()
	} else {
		b, err = ChainBadaCtx(&b)
		if err != nil {
			log.Print("Warning: previous badaCtx is invalid, both curr and entry id should be exist. Now using new badaCtx")
			b = NewBadaCtx()
		}
	}

	// error is ignored because b is always valid here
	ctx, _ = b.EmbedIntoContext(ctx)
	c.Request = c.Request.WithContext(ctx)

	c.Next()
}