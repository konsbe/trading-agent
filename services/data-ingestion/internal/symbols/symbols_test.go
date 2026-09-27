package symbols

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRows struct {
	vals []string
	i    int
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Next() bool                                   { r.i++; return r.i <= len(r.vals) }
func (r *fakeRows) Scan(dest ...any) error                       { *dest[0].(*string) = r.vals[r.i-1]; return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

type fakeQ struct {
	vals []string
	err  error
	sql  string
}

func (f *fakeQ) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	f.sql = sql
	if f.err != nil {
		return nil, f.err
	}
	return &fakeRows{vals: f.vals}, nil
}

func logger() (*slog.Logger, *bytes.Buffer) {
	var b bytes.Buffer
	return slog.New(slog.NewTextHandler(&b, nil)), &b
}

func TestFollowedUsesTheTableAndNormalises(t *testing.T) {
	log, buf := logger()
	got := Followed(context.Background(), &fakeQ{vals: []string{"xom", " SPY "}}, log, "svc", "list", []string{"equity"}, []string{"ENV"})
	if strings.Join(got, ",") != "XOM,SPY" || buf.Len() != 0 {
		t.Errorf("got %v, log %q", got, buf.String())
	}
}

func TestFallbackIsLoudAndNamesServiceAndList(t *testing.T) {
	for name, q := range map[string]*fakeQ{
		"unreachable": {err: errors.New("dial tcp: connection refused")},
		"empty":       {},
	} {
		log, buf := logger()
		got := Computation(context.Background(), q, log, "data-technical", "equity bars (TECHNICAL_EQUITY_SYMBOLS)", []string{"equity"}, []string{"AAPL", "MSFT"})
		out := buf.String()
		if strings.Join(got, ",") != "AAPL,MSFT" {
			t.Errorf("%s: got %v, want the .env fallback", name, got)
		}
		for _, want := range []string{"SYMBOL LIST FALLBACK", "service=data-technical", "TECHNICAL_EQUITY_SYMBOLS"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: log %q lacks %q", name, out, want)
			}
		}
		level := map[string]string{"unreachable": "level=ERROR", "empty": "level=WARN"}[name]
		if !strings.Contains(out, level) {
			t.Errorf("%s: log %q, want %s", name, out, level)
		}
	}
}

func TestComputationIncludesOpenInterest(t *testing.T) {
	q := &fakeQ{vals: []string{"AESI"}}
	log, _ := logger()
	Computation(context.Background(), q, log, "s", "l", []string{"equity"}, nil)
	if !strings.Contains(q.sql, "computation_interest") || !strings.Contains(q.sql, "active_until IS NULL") {
		t.Errorf("computation set query = %s", q.sql)
	}
}
