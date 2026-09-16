package dbprovider

import (
	"errors"
	"testing"
)

type stubProvider struct {
	rec Record
	err error
}

func (s stubProvider) Lookup(string) (Record, error) { return s.rec, s.err }
func (s stubProvider) Close() error                  { return nil }

func TestCombined_FirstNonEmptyWins(t *testing.T) {
	c := NewCombined([]Named{
		{Key: "b_ipinfo", Provider: stubProvider{rec: Record{Country: "US", Asn: "AS1"}}},
		{Key: "a_maxmind", Provider: stubProvider{rec: Record{Country: "GB", City: "London"}}},
	})
	got, err := c.Lookup("8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}
	if got.Country != "GB" {
		t.Errorf("country: got %q want GB (a_maxmind first)", got.Country)
	}
	if got.City != "London" {
		t.Errorf("city: got %q", got.City)
	}
	if got.Asn != "AS1" {
		t.Errorf("asn: got %q want AS1", got.Asn)
	}
}

func TestCombined_SkipErrorThenFill(t *testing.T) {
	c := NewCombined([]Named{
		{Key: "a", Provider: stubProvider{err: errors.New("missing")}},
		{Key: "b", Provider: stubProvider{rec: Record{Country: "US"}}},
	})
	got, err := c.Lookup("1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Country != "US" {
		t.Errorf("country: got %q", got.Country)
	}
}

func TestCombined_AllErrors(t *testing.T) {
	c := NewCombined([]Named{
		{Key: "a", Provider: stubProvider{err: errors.New("one")}},
		{Key: "b", Provider: stubProvider{err: errors.New("two")}},
	})
	_, err := c.Lookup("1.1.1.1")
	if err == nil || err.Error() != "one" {
		t.Fatalf("got %v want first error", err)
	}
}
