package cmd

import (
	"errors"
	"testing"
)

func TestBlocking_Nil(t *testing.T) {
	err := Blocking(nil)
	if err != nil {
		t.Errorf("Blocking(nil) should return nil, got %v", err)
	}
}

func TestBlocking_WrapsError(t *testing.T) {
	original := errors.New("test error")
	err := Blocking(original)
	if err == nil {
		t.Error("Blocking(err) should return non-nil error")
	}
	if !IsBlocking(err) {
		t.Error("Blocking(err) should satisfy IsBlocking")
	}
	if !errors.Is(err, original) {
		t.Error("Blocking(err) should satisfy errors.Is for original error")
	}
}

func TestIsBlocking_TrueForBlockedError(t *testing.T) {
	err := Blocking(errors.New("test"))
	if !IsBlocking(err) {
		t.Error("IsBlocking should return true for Blocking-wrapped error")
	}
}

func TestIsBlocking_FalseForPlainError(t *testing.T) {
	err := errors.New("plain error")
	if IsBlocking(err) {
		t.Error("IsBlocking should return false for plain error")
	}
}

func TestUnwrap(t *testing.T) {
	original := errors.New("original error")
	err := Blocking(original)
	unwrapped := errors.Unwrap(err)
	if unwrapped != original {
		t.Errorf("errors.Unwrap should return original error, got %v", unwrapped)
	}
}
