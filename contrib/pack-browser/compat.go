package browser

import (
	"context"

	"github.com/chromedp/chromedp"
)

// action is a chromedp step that yields no value.
type action = chromedp.Action[chromedp.Void]

// into runs a value-returning chromedp action and stores its result in dst,
// so that it can be composed in a list of void actions.
func into[T any](dst *T, a chromedp.Action[T]) action {
	return func(ctx context.Context, t *chromedp.Target) (chromedp.Void, error) {
		v, err := a(ctx, t)
		if err != nil {
			return chromedp.Void{}, err
		}
		*dst = v
		return chromedp.Void{}, nil
	}
}

// discard drops the value of a value-returning chromedp action.
func discard[T any](a chromedp.Action[T]) action {
	return func(ctx context.Context, t *chromedp.Target) (chromedp.Void, error) {
		_, err := a(ctx, t)
		return chromedp.Void{}, err
	}
}

// attrInto stores the value and existence of an attribute lookup.
func attrInto(value *string, ok *bool, a chromedp.Action[chromedp.AttributeResult]) action {
	var r chromedp.AttributeResult
	set := into(&r, a)
	return func(ctx context.Context, t *chromedp.Target) (chromedp.Void, error) {
		if _, err := set(ctx, t); err != nil {
			return chromedp.Void{}, err
		}
		*value, *ok = r.Value, r.Exists
		return chromedp.Void{}, nil
	}
}
