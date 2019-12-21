package auth

import (
	"context"
	"fmt"
	"testing"
	"time"
)

const simplePolicy = `package policy
decision := true`

const dataPolicy = `package policy
import input as request

default decision = false

decision {
    public_destination
}
decision {
    allowed_by_ip
}

allowed_by_ip {
    request.source_address = data.allowed_ips[_]
}

public_destination {
    request.destination = data.public_sites[_]
}
`

func TestAuthorize(t *testing.T) {
	ctx := context.Background()
	s := New()
	if err := s.LoadPolicy(ctx, simplePolicy); err != nil {
		t.Errorf("load policy: %v", err)
	}
	ok, err := s.Eval(ctx, nil)
	if err != nil {
		t.Errorf("eval: %v", err)
	}
	if got, want := ok, true; got != want {
		t.Errorf("final decision:\n  got: %v\n want: %v", got, want)
	}
}

func TestLoadData(t *testing.T) {
	ctx := context.Background()
	s := New()
	if err := s.LoadPolicy(ctx, dataPolicy); err != nil {
		t.Errorf("load policy: %v", err)
	}
	if err := s.AddData(ctx, "/allowed_ips", []string{"8.8.8.8", "4.4.4.4"}); err != nil {
		t.Errorf("add data: %v", err)
	}

	request := map[string]string{"source_address": "1.2.3.4"}
	ok, err := s.Eval(ctx, request)
	if err != nil {
		t.Errorf("eval: %v", err)
	}
	if got, want := ok, false; got != want {
		t.Errorf("decision:\n  got: %v\n want: %v", got, want)
	}

	if err := s.AddData(ctx, "/allowed_ips", []string{"1.2.3.4"}); err != nil {
		t.Errorf("change data: %v", err)
	}
	ok, err = s.Eval(ctx, request)
	if err != nil {
		t.Errorf("eval: %v", err)
	}
	if got, want := ok, true; got != want {
		t.Errorf("decision:\n  got: %v\n want: %v", got, want)
	}
}

const initialRacePolicy = `package policy

default decision = false
`

const secondRacePolicy = `
package policy

default decision = false

decision {
    input > 500000
}
`

const finalRacePolicy = `package policy

default decision = false

decision {
    input == data.special_numbers[_]
}
`

func TestRace(t *testing.T) {
	ctx := context.Background()
	s := New()
	if err := s.LoadPolicy(ctx, initialRacePolicy); err != nil {
		t.Fatalf("load policy: %v", err)
	}
	evalCh := make(chan error)
	go func() {
		var gotTrue, gotFalse int
		for i := 0; i < 100000; i++ {
			ok, err := s.Eval(ctx, i)
			if err != nil {
				evalCh <- err
			}
			if ok {
				gotTrue++
			} else {
				gotFalse++
			}
			if gotTrue > 0 && gotFalse > 0 {
				evalCh <- nil
				return
			}
		}
		evalCh <- fmt.Errorf("did not get both types of decision: true %d, false %d", gotTrue, gotFalse)
	}()

	if err := s.LoadPolicy(ctx, secondRacePolicy); err != nil {
		t.Fatalf("load second policy: %v", err)
	}

	numbersA := []int{20000, 40000, 60000, 80000, 99999}
	numbersB := []int{10000, 30000, 50000, 70000, 90000, 99999}

	timeout := time.After(20 * time.Second) // emergency timeout, should never be hit
loop:
	for i := 0; ; i++ {
		if i == 10000 {
			if err := s.LoadPolicy(ctx, finalRacePolicy); err != nil {
				t.Fatalf("load final policy: %v", err)
			}
			continue
		}

		var copy []int
		if i%2 == 0 {
			copy = append(copy, numbersA...)
		} else {
			copy = append(copy, numbersB...)
		}
		if err := s.AddData(ctx, "/special_numbers", copy); err != nil {
			t.Fatalf("update special numbers: %v", err)
		}
		select {
		case err := <-evalCh:
			if err != nil {
				t.Fatalf("background evaluation: %v", err)
			} else {
				break loop
			}
		case <-timeout:
			t.Fatal("timeout")
		default:
		}
	}
}
