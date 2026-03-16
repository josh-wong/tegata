package guard

import (
	"bytes"
	"testing"
)

func TestNewSecretBuffer_BytesReturnsData(t *testing.T) {
	data := []byte("secret")
	sb := NewSecretBuffer(data)
	defer sb.Destroy()

	got := sb.Bytes()
	if !bytes.Equal(got, []byte("secret")) {
		t.Errorf("Bytes() = %q, want %q", got, "secret")
	}
}

func TestNewSecretBuffer_InputWiped(t *testing.T) {
	data := []byte("sensitive")
	original := make([]byte, len(data))
	copy(original, data)

	_ = NewSecretBuffer(data)

	// After NewSecretBuffer, the input slice should be zeroed.
	zeroed := make([]byte, len(original))
	if !bytes.Equal(data, zeroed) {
		t.Errorf("input data was not wiped: got %v, want all zeros", data)
	}
}

func TestSecretBuffer_Destroy(t *testing.T) {
	sb := NewSecretBuffer([]byte("to-destroy"))

	sb.Destroy()

	// After Destroy(), Bytes() should return nil.
	got := sb.Bytes()
	if got != nil {
		t.Errorf("Bytes() after Destroy() = %v, want nil", got)
	}
}

func TestSealOpen_RoundTrip(t *testing.T) {
	original := []byte("round-trip-data")
	sb := NewSecretBuffer(original)

	ke := Seal(sb)

	opened, err := ke.Open()
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer opened.Destroy()

	if !bytes.Equal(opened.Bytes(), []byte("round-trip-data")) {
		t.Errorf("Open() = %q, want %q", opened.Bytes(), "round-trip-data")
	}
}

func TestKeyEnclave_Open_MultipleTimes(t *testing.T) {
	sb := NewSecretBuffer([]byte("multi-open"))
	ke := Seal(sb)

	for i := 0; i < 3; i++ {
		opened, err := ke.Open()
		if err != nil {
			t.Fatalf("Open() attempt %d error: %v", i+1, err)
		}
		if !bytes.Equal(opened.Bytes(), []byte("multi-open")) {
			t.Errorf("Open() attempt %d = %q, want %q", i+1, opened.Bytes(), "multi-open")
		}
		opened.Destroy()
	}
}

func TestNewSecretBufferFromSize(t *testing.T) {
	sb := NewSecretBufferFromSize(32)
	defer sb.Destroy()

	if sb.Size() != 32 {
		t.Errorf("Size() = %d, want 32", sb.Size())
	}

	// Should be zeroed.
	expected := make([]byte, 32)
	if !bytes.Equal(sb.Bytes(), expected) {
		t.Errorf("Bytes() = %v, want all zeros", sb.Bytes())
	}
}
