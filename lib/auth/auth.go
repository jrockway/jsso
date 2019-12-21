// Package auth handles authorizing HTTP requests.
package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	opentracing "github.com/opentracing/opentracing-go"
)

type Server struct {
	store storage.Store

	queryMu  sync.Mutex
	hasQuery bool
	query    rego.PreparedEvalQuery
}

const bootstrap = `package policy

decision := {}
`

// New creates a new evaluation engine.  It is safe to use all operations concurrently from multiple
// goroutines.
func New() *Server {
	s := new(Server)
	s.store = inmem.New()
	return s
}

// LoadPolicy replaces the authorization policy with the provided rego code.
func (s *Server) LoadPolicy(ctx context.Context, code string) error {
	sp, ctx := opentracing.StartSpanFromContext(ctx, "load_policy")
	defer sp.Finish()

	r := rego.New(
		rego.Store(s.store),
		rego.Module("", code),
		rego.Query("data.policy.decision"),
	)

	q, err := r.PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("prepare policy: %w", err)
	}

	s.queryMu.Lock()
	s.query = q
	s.hasQuery = true
	s.queryMu.Unlock()

	return nil
}

// Eval returns an authorization decision based on the input, policy, and stored data.  An error
// indicates a technical problem evaluting the policy.  The result is only valid if an error did not
// occur.
func (s *Server) Eval(ctx context.Context, input interface{}) (bool, error) {
	s.queryMu.Lock()
	q := s.query
	hasQuery := s.hasQuery
	s.queryMu.Unlock()
	if !hasQuery {
		return false, errors.New("no policy loaded")
	}
	rs, err := q.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("eval query: %w", err)
	}
	if len(rs) < 1 {
		return false, errors.New("no result produced")
	}
	if len(rs) > 1 {
		return false, fmt.Errorf("too many results: got %d, want 1", len(rs))
	}
	expressions := rs[0].Expressions
	if got, want := len(expressions), 1; got != want {
		return false, fmt.Errorf("unexpected expression count: got %d, want %d", got, want)
	}
	switch decision := expressions[0].Value.(type) {
	case bool:
		return decision, nil
	default:
		return false, fmt.Errorf("non-boolean decision generated: %v", decision)
	}
}

// AddData adds or replaces data used to make policy decisions.
func (s *Server) AddData(ctx context.Context, rawPath string, value interface{}) error {
	sp, ctx := opentracing.StartSpanFromContext(ctx, "add data", opentracing.Tag{Key: "path", Value: rawPath})
	defer sp.Finish()

	path, ok := storage.ParsePathEscaped(rawPath)
	if !ok {
		return fmt.Errorf("problem parsing path %q", rawPath)
	}

	txn, err := s.store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	if _, err := s.store.Read(ctx, txn, path); err != nil {
		if !storage.IsNotFound(err) {
			s.store.Abort(ctx, txn)
			return fmt.Errorf("reading path %q: %w", rawPath, err)
		}
		if err := storage.MakeDir(ctx, s.store, txn, path[:len(path)-1]); err != nil {
			s.store.Abort(ctx, txn)
			return fmt.Errorf("creating directory for path %q: %w", rawPath, err)
		}
	}
	if err := s.store.Write(ctx, txn, storage.AddOp, path, value); err != nil {
		s.store.Abort(ctx, txn)
		return fmt.Errorf("adding value: %w", err)
	}
	if err := s.store.Commit(ctx, txn); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
