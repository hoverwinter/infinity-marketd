package marketdata

import (
	"context"
	"fmt"
	"reflect"
	"sort"
)

// Registry is configured once at startup. No global registration or auto fallback.
type Registry struct{ providers map[string]Provider }

func NewRegistry(providers ...Provider) (*Registry, error) {
	r := &Registry{providers: make(map[string]Provider, len(providers))}
	for _, p := range providers {
		if p == nil || (reflect.ValueOf(p).Kind() == reflect.Pointer && reflect.ValueOf(p).IsNil()) {
			return nil, fmt.Errorf("%w: nil provider", ErrInvalid)
		}
		if !identifier.MatchString(p.ID()) {
			return nil, fmt.Errorf("%w: invalid provider ID", ErrInvalid)
		}
		if _, ok := r.providers[p.ID()]; ok {
			return nil, fmt.Errorf("%w: duplicate provider %q", ErrInvalid, p.ID())
		}
		r.providers[p.ID()] = p
	}
	return r, nil
}

// WithProvider returns a new registry with one source replaced/added, preserving
// every other source. Configure before serving; the original remains unchanged.
func (r *Registry) WithProvider(provider Provider) (*Registry, error) {
	next, err := NewRegistry(provider)
	if err != nil {
		return nil, err
	}
	for id, p := range r.providers {
		if id != provider.ID() {
			next.providers[id] = p
		}
	}
	return next, nil
}

func (r *Registry) Providers() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(r.providers))
	for id, p := range r.providers {
		info := ProviderInfo{ID: id, Bars: []BarsCapability{}, BoardKinds: []string{}}
		if bp, ok := p.(BarsProvider); ok {
			info.Bars = bp.BarsCapabilities()
		}
		if bp, ok := p.(BoardProvider); ok {
			info.BoardKinds = bp.BoardKinds()
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) get(id string) (Provider, error) {
	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("%w: provider %q", ErrNotFound, id)
	}
	return p, nil
}

func (r *Registry) Bars(ctx context.Context, id string, q BarsQuery) (BarsResult, error) {
	p, err := r.get(id)
	if err != nil {
		return BarsResult{}, err
	}
	bp, ok := p.(BarsProvider)
	if !ok {
		return BarsResult{}, fmt.Errorf("%w: %s bars", ErrUnsupported, id)
	}
	return bp.Bars(ctx, q)
}

func (r *Registry) Boards(ctx context.Context, id, kind string) (BoardsResult, error) {
	p, err := r.get(id)
	if err != nil {
		return BoardsResult{}, err
	}
	bp, ok := p.(BoardProvider)
	if !ok {
		return BoardsResult{}, fmt.Errorf("%w: %s boards", ErrUnsupported, id)
	}
	return bp.Boards(ctx, kind)
}

func (r *Registry) ResolveBoard(ctx context.Context, id, kind, code string) (BoardResult, error) {
	p, err := r.get(id)
	if err != nil {
		return BoardResult{}, err
	}
	bp, ok := p.(BoardProvider)
	if !ok {
		return BoardResult{}, fmt.Errorf("%w: %s board resolution", ErrUnsupported, id)
	}
	return bp.ResolveBoard(ctx, kind, code)
}
