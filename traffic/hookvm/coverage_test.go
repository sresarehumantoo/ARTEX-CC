package hookvm

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func compileAndRun(t *testing.T, phase, src string, req *Request, resp *Response) error {
	t.Helper()
	prog, err := Compile(phase, src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if phase == "request" {
		return prog.RunRequest(context.Background(), req, RunOptions{})
	}
	return prog.RunResponse(context.Background(), req, resp, RunOptions{})
}

func mustRun(t *testing.T, phase, src string, req *Request, resp *Response) {
	t.Helper()
	if err := compileAndRun(t, phase, src, req, resp); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func defaultReq() *Request {
	return &Request{
		Scheme: "https", Host: "example.test", Port: "443",
		Method: "GET", Path: "/test", RawQuery: "a=1",
		Headers: map[string][]string{},
		Body:    []byte("hello"),
	}
}

func TestEvalIndex(t *testing.T) {
	t.Run("map_string_any", func(t *testing.T) {
		src := `package hook
import "encoding/json"
func OnRequest(req *Request) error {
	v, err := json.Decode(req.Body)
	if err != nil { return err }
	m, _ := v.(map[string]any)
	name, _ := m["name"].(string)
	req.SetHeader("X-Name", name)
	return nil
}`
		req := defaultReq()
		req.Body = []byte(`{"name":"alice"}`)
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Name"]; len(got) != 1 || got[0] != "alice" {
			t.Fatalf("X-Name = %v", got)
		}
	})

	t.Run("slice_any", func(t *testing.T) {
		src := `package hook
import "strings"
func OnRequest(req *Request) error {
	parts := strings.Split(req.Path, "/")
	req.SetHeader("X-Part", parts[2])
	return nil
}`
		req := defaultReq()
		req.Path = "/api/users"
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Part"]; len(got) != 1 || got[0] != "users" {
			t.Fatalf("X-Part = %v", got)
		}
	})

	t.Run("byte_slice", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	b := req.Body
	v := b[0]
	if v == 104 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
		req := defaultReq()
		req.Body = []byte("hello")
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("string_index", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	s := "ABCDE"
	ch := s[2]
	if ch == 67 {
		req.SetHeader("X-Ch", "C")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ch"]; len(got) != 1 || got[0] != "C" {
			t.Fatalf("X-Ch = %v", got)
		}
	})

	t.Run("map_comma_ok", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	m := map[string]any{"a": "1"}
	v, ok := m["a"]
	if ok {
		req.SetHeader("X-V", v.(string))
	}
	_, miss := m["z"]
	if !miss {
		req.SetHeader("X-Miss", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "1" {
			t.Fatalf("X-V = %v", got)
		}
		if got := req.Headers["X-Miss"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Miss = %v", got)
		}
	})

	t.Run("headers_map_index", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	vals := req.Headers["X-In"]
	req.SetHeader("X-Got", vals[0])
	return nil
}`
		req := defaultReq()
		req.Headers["X-In"] = []string{"fromheader"}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Got"]; len(got) != 1 || got[0] != "fromheader" {
			t.Fatalf("X-Got = %v", got)
		}
	})
}

func TestEvalSlice(t *testing.T) {
	t.Run("string_slice", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	s := "Hello, World!"
	req.SetHeader("X-Sub", s[0:5])
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Sub"]; len(got) != 1 || got[0] != "Hello" {
			t.Fatalf("X-Sub = %v", got)
		}
	})

	t.Run("byte_slice_expr", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	b := req.Body
	req.Body = b[1:4]
	return nil
}`
		req := defaultReq()
		req.Body = []byte("abcdef")
		mustRun(t, "request", src, req, nil)
		if string(req.Body) != "bcd" {
			t.Fatalf("Body = %q, want bcd", req.Body)
		}
	})

	t.Run("any_slice_expr", func(t *testing.T) {
		src := `package hook
import "strings"
func OnRequest(req *Request) error {
	parts := strings.Split("a,b,c,d", ",")
	sub := parts[1:3]
	req.SetHeader("X-Len", "2")
	if len(sub) == 2 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("slice_defaults", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	s := "ABCDE"
	req.SetHeader("X-Lo", s[:3])
	req.SetHeader("X-Hi", s[3:])
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Lo"]; len(got) != 1 || got[0] != "ABC" {
			t.Fatalf("X-Lo = %v", got)
		}
		if got := req.Headers["X-Hi"]; len(got) != 1 || got[0] != "DE" {
			t.Fatalf("X-Hi = %v", got)
		}
	})
}

func TestZeroValueAndVarDecl(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	var s string
	var n int
	var f float64
	var b bool
	var m map[string]any
	var bs []byte
	var sl []any

	req.SetHeader("X-Str", s)
	req.SetHeader("X-Num", strconv.Itoa(n))
	if f == 0.0 {
		req.SetHeader("X-Flt", "zero")
	}
	if !b {
		req.SetHeader("X-Bool", "false")
	}
	if len(m) == 0 {
		req.SetHeader("X-Map", "empty")
	}
	if len(bs) == 0 {
		req.SetHeader("X-Bytes", "empty")
	}
	if len(sl) == 0 {
		req.SetHeader("X-Slice", "empty")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	checks := map[string]string{
		"X-Str": "", "X-Num": "0", "X-Flt": "zero", "X-Bool": "false",
		"X-Map": "empty", "X-Bytes": "empty", "X-Slice": "empty",
	}
	for k, want := range checks {
		if got := req.Headers[k]; len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want %q", k, got, want)
		}
	}
}

func TestCallBuiltins(t *testing.T) {
	t.Run("len_various_types", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	req.SetHeader("X-Lenstr", strconv.Itoa(len("abcde")))
	req.SetHeader("X-Lenbody", strconv.Itoa(len(req.Body)))
	m := map[string]any{"a": "1", "b": "2"}
	req.SetHeader("X-Lenmap", strconv.Itoa(len(m)))
	sl := make([]any, 3)
	req.SetHeader("X-Lenarr", strconv.Itoa(len(sl)))
	req.SetHeader("X-Lenhdr", strconv.Itoa(len(req.Headers)))
	return nil
}`
		req := defaultReq()
		req.Body = []byte("test")
		req.Headers["X-One"] = []string{"v"}
		mustRun(t, "request", src, req, nil)
		checks := map[string]string{
			"X-Lenstr": "5", "X-Lenbody": "4", "X-Lenmap": "2", "X-Lenarr": "3", "X-Lenhdr": "5",
		}
		for k, want := range checks {
			if got := req.Headers[k]; len(got) == 0 || got[0] != want {
				t.Errorf("%s = %v, want %q", k, got, want)
			}
		}
	})

	t.Run("append_bytes", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	b := []byte{72, 73}
	b = append(b, 33)
	extra := []byte{63}
	b = append(b, extra)
	req.Body = b
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if string(req.Body) != "HI!?" {
			t.Fatalf("Body = %q, want HI!?", req.Body)
		}
	})

	t.Run("append_any_slice", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sl := make([]any, 0)
	sl = append(sl, "a")
	sl = append(sl, "b")
	req.SetHeader("X-Len", strconv.Itoa(len(sl)))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "2" {
			t.Fatalf("X-Len = %v", got)
		}
	})

	t.Run("delete_map", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	m := map[string]any{"a": "1", "b": "2", "c": "3"}
	delete(m, "b")
	req.SetHeader("X-Len", strconv.Itoa(len(m)))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "2" {
			t.Fatalf("X-Len = %v", got)
		}
	})

	t.Run("delete_header_map", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	delete(req.Headers, "X-Kill")
	req.SetHeader("X-Remain", strconv.Itoa(len(req.Headers)))
	return nil
}`
		req := defaultReq()
		req.Headers["X-Kill"] = []string{"v"}
		req.Headers["X-Keep"] = []string{"v"}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Remain"]; len(got) == 0 || got[0] != "1" {
			t.Fatalf("X-Remain = %v", got)
		}
	})

	t.Run("string_conversion", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.SetHeader("X-S", string(req.Body))
	req.SetHeader("X-R", string(65))
	return nil
}`
		req := defaultReq()
		req.Body = []byte("data")
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-S"]; len(got) != 1 || got[0] != "data" {
			t.Fatalf("X-S = %v", got)
		}
		if got := req.Headers["X-R"]; len(got) != 1 || got[0] != "A" {
			t.Fatalf("X-R = %v", got)
		}
	})

	t.Run("numeric_conversions", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	x := 42
	req.SetHeader("X-I", strconv.Itoa(int(x)))
	req.SetHeader("X-I64", strconv.Itoa(int64(x)))
	b := byte(65)
	req.SetHeader("X-B", string(b))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-I"]; len(got) != 1 || got[0] != "42" {
			t.Fatalf("X-I = %v", got)
		}
		if got := req.Headers["X-B"]; len(got) != 1 || got[0] != "A" {
			t.Fatalf("X-B = %v", got)
		}
	})

	t.Run("float64_conversion", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	n := 42
	f := float64(n)
	if f == 42.0 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("panic_no_args", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	panic("custom boom")
}`
		prog, err := Compile("request", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunRequest(context.Background(), defaultReq(), RunOptions{})
		var pe *PanicError
		if !errors.As(err, &pe) {
			t.Fatalf("expected PanicError, got %v", err)
		}
	})
}

func TestBinaryExpressions(t *testing.T) {
	t.Run("integer_arithmetic", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	a := 10
	b := 3
	req.SetHeader("X-Add", strconv.Itoa(a + b))
	req.SetHeader("X-Sub", strconv.Itoa(a - b))
	req.SetHeader("X-Mul", strconv.Itoa(a * b))
	req.SetHeader("X-Div", strconv.Itoa(a / b))
	req.SetHeader("X-Mod", strconv.Itoa(a % b))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		checks := map[string]string{
			"X-Add": "13", "X-Sub": "7", "X-Mul": "30", "X-Div": "3", "X-Mod": "1",
		}
		for k, want := range checks {
			if got := req.Headers[k]; len(got) != 1 || got[0] != want {
				t.Errorf("%s = %v, want %q", k, got, want)
			}
		}
	})

	t.Run("float_arithmetic", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	a := 10.5
	b := 2.0
	sum := a + b
	diff := a - b
	prod := a * b
	quot := a / b
	if sum == 12.5 && diff == 8.5 && prod == 21.0 && quot == 5.25 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("comparisons", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	if 1 < 2 {
		req.SetHeader("X-Lt", "yes")
	}
	if 2 <= 2 {
		req.SetHeader("X-Le", "yes")
	}
	if 3 > 2 {
		req.SetHeader("X-Gt", "yes")
	}
	if 3 >= 3 {
		req.SetHeader("X-Ge", "yes")
	}
	if 1 == 1 {
		req.SetHeader("X-Eq", "yes")
	}
	if 1 != 2 {
		req.SetHeader("X-Ne", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		for _, k := range []string{"X-Lt", "X-Le", "X-Gt", "X-Ge", "X-Eq", "X-Ne"} {
			if got := req.Headers[k]; len(got) != 1 || got[0] != "yes" {
				t.Errorf("%s = %v, want yes", k, got)
			}
		}
	})

	t.Run("string_comparisons", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	if "abc" < "def" {
		req.SetHeader("X-Lt", "yes")
	}
	if "abc" <= "abc" {
		req.SetHeader("X-Le", "yes")
	}
	if "z" > "a" {
		req.SetHeader("X-Gt", "yes")
	}
	if "z" >= "z" {
		req.SetHeader("X-Ge", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		for _, k := range []string{"X-Lt", "X-Le", "X-Gt", "X-Ge"} {
			if got := req.Headers[k]; len(got) != 1 || got[0] != "yes" {
				t.Errorf("%s = %v, want yes", k, got)
			}
		}
	})

	t.Run("float_comparisons", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	if 1.5 < 2.5 {
		req.SetHeader("X-Lt", "yes")
	}
	if 2.5 >= 2.5 {
		req.SetHeader("X-Ge", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Lt"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Lt = %v", got)
		}
		if got := req.Headers["X-Ge"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ge = %v", got)
		}
	})

	t.Run("string_concat", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	s := "Hello" + ", " + "World"
	req.SetHeader("X-S", s)
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-S"]; len(got) != 1 || got[0] != "Hello, World" {
			t.Fatalf("X-S = %v", got)
		}
	})

	t.Run("nil_equality", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	var v any
	if v == nil {
		req.SetHeader("X-Nil", "yes")
	}
	v = "hello"
	if v != nil {
		req.SetHeader("X-Notnull", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Nil"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Nil = %v", got)
		}
		if got := req.Headers["X-Notnull"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Notnull = %v", got)
		}
	})

	t.Run("logical_operators", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	if true && true {
		req.SetHeader("X-And", "yes")
	}
	if false || true {
		req.SetHeader("X-Or", "yes")
	}
	if !(false) {
		req.SetHeader("X-Not", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		for _, k := range []string{"X-And", "X-Or", "X-Not"} {
			if got := req.Headers[k]; len(got) != 1 || got[0] != "yes" {
				t.Errorf("%s = %v", k, got)
			}
		}
	})

	t.Run("division_by_zero", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	x := 10 / 0
	_ = x
	return nil
}`
		prog, err := Compile("request", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunRequest(context.Background(), defaultReq(), RunOptions{})
		if err == nil || !strings.Contains(err.Error(), "divide by zero") {
			t.Fatalf("expected divide by zero error, got %v", err)
		}
	})

	t.Run("float_division_by_zero", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	x := 10.0 / 0.0
	_ = x
	return nil
}`
		prog, err := Compile("request", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunRequest(context.Background(), defaultReq(), RunOptions{})
		if err == nil || !strings.Contains(err.Error(), "division by zero") {
			t.Fatalf("expected division by zero error, got %v", err)
		}
	})
}

func TestUnaryExpressions(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	n := -42
	req.SetHeader("X-Neg", strconv.Itoa(n))
	p := +5
	req.SetHeader("X-Pos", strconv.Itoa(p))
	f := -3.14
	if f < 0.0 {
		req.SetHeader("X-Negfloat", "yes")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Neg"]; len(got) != 1 || got[0] != "-42" {
		t.Fatalf("X-Neg = %v", got)
	}
	if got := req.Headers["X-Pos"]; len(got) != 1 || got[0] != "5" {
		t.Fatalf("X-Pos = %v", got)
	}
	if got := req.Headers["X-Negfloat"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Negfloat = %v", got)
	}
}

func TestIncDec(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	x := 5
	x++
	req.SetHeader("X-Inc", strconv.Itoa(x))
	x--
	x--
	req.SetHeader("X-Dec", strconv.Itoa(x))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Inc"]; len(got) != 1 || got[0] != "6" {
		t.Fatalf("X-Inc = %v", got)
	}
	if got := req.Headers["X-Dec"]; len(got) != 1 || got[0] != "4" {
		t.Fatalf("X-Dec = %v", got)
	}
}

func TestCompoundAssignments(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	x := 10
	x += 5
	req.SetHeader("X-Add", strconv.Itoa(x))
	x -= 3
	req.SetHeader("X-Sub", strconv.Itoa(x))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Add"]; len(got) != 1 || got[0] != "15" {
		t.Fatalf("X-Add = %v", got)
	}
	if got := req.Headers["X-Sub"]; len(got) != 1 || got[0] != "12" {
		t.Fatalf("X-Sub = %v", got)
	}
}

func TestForLoop(t *testing.T) {
	t.Run("classic_for", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sum := 0
	for i := 0; i < 5; i++ {
		sum += i
	}
	req.SetHeader("X-Sum", strconv.Itoa(sum))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Sum"]; len(got) != 1 || got[0] != "10" {
			t.Fatalf("X-Sum = %v", got)
		}
	})

	t.Run("for_break_continue", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sum := 0
	for i := 0; i < 10; i++ {
		if i == 7 {
			break
		}
		if i % 2 == 0 {
			continue
		}
		sum += i
	}
	req.SetHeader("X-Sum", strconv.Itoa(sum))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Sum"]; len(got) != 1 || got[0] != "9" {
			t.Fatalf("X-Sum = %v, want 9", got)
		}
	})

	t.Run("for_return", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	for i := 0; i < 10; i++ {
		if i == 3 {
			req.SetHeader("X-At", "3")
			return nil
		}
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-At"]; len(got) != 1 || got[0] != "3" {
			t.Fatalf("X-At = %v", got)
		}
	})
}

func TestRangeStatement(t *testing.T) {
	t.Run("range_bytes", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sum := 0
	for _, v := range req.Body {
		sum += v
	}
	req.SetHeader("X-Sum", strconv.Itoa(sum))
	return nil
}`
		req := defaultReq()
		req.Body = []byte{1, 2, 3}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Sum"]; len(got) != 1 || got[0] != "6" {
			t.Fatalf("X-Sum = %v", got)
		}
	})

	t.Run("range_string", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	count := 0
	for _, _ = range "abc" {
		count++
	}
	req.SetHeader("X-Count", strconv.Itoa(count))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Count"]; len(got) != 1 || got[0] != "3" {
			t.Fatalf("X-Count = %v", got)
		}
	})

	t.Run("range_string_slice", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	vals := req.Headers["X-In"]
	joined := ""
	for _, v := range vals {
		joined += v
	}
	req.SetHeader("X-Out", joined)
	return nil
}`
		req := defaultReq()
		req.Headers["X-In"] = []string{"a", "b", "c"}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Out"]; len(got) != 1 || got[0] != "abc" {
			t.Fatalf("X-Out = %v", got)
		}
	})

	t.Run("range_nil", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	var m map[string]any
	for _, _ = range m {
		req.SetHeader("X-Bad", "1")
	}
	req.SetHeader("X-Ok", "yes")
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("range_break", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	items := make([]any, 5)
	for i := 0; i < 5; i++ {
		items[i] = i
	}
	count := 0
	for _, _ = range items {
		count++
		if count == 3 {
			break
		}
	}
	req.SetHeader("X-Count", strconv.Itoa(count))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Count"]; len(got) != 1 || got[0] != "3" {
			t.Fatalf("X-Count = %v", got)
		}
	})

	t.Run("range_map_sorted", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	m := map[string]any{"c": "3", "a": "1", "b": "2"}
	keys := ""
	for k, _ := range m {
		keys += k
	}
	req.SetHeader("X-Keys", keys)
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Keys"]; len(got) != 1 || got[0] != "abc" {
			t.Fatalf("X-Keys = %v, want abc", got)
		}
	})

	t.Run("range_headers_map", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	count := 0
	for _, _ = range req.Headers {
		count++
	}
	if count > 0 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
		req := defaultReq()
		req.Headers["X-One"] = []string{"v"}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})
}

func TestTypeAssertions(t *testing.T) {
	src := `package hook
import "encoding/json"
func OnRequest(req *Request) error {
	v, err := json.Decode(req.Body)
	if err != nil {
		return err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	name, ok := m["name"].(string)
	if ok {
		req.SetHeader("X-Name", name)
	}
	active, ok := m["active"].(bool)
	if ok {
		if active {
			req.SetHeader("X-Active", "true")
		}
	}
	score, ok := m["score"].(float64)
	if ok && score > 90.0 {
		req.SetHeader("X-Score", "high")
	}
	items, ok := m["items"].([]any)
	if ok {
		if len(items) > 0 {
			req.SetHeader("X-Items", "yes")
		}
	}
	return nil
}`
	req := defaultReq()
	req.Body = []byte(`{"name":"bob","active":true,"score":95.5,"items":["a"]}`)
	mustRun(t, "request", src, req, nil)
	checks := map[string]string{
		"X-Name": "bob", "X-Active": "true", "X-Score": "high", "X-Items": "yes",
	}
	for k, want := range checks {
		if got := req.Headers[k]; len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want %q", k, got, want)
		}
	}
}

func TestTypeAssertByteSlice(t *testing.T) {
	src := `package hook
import "encoding/base64"
func OnRequest(req *Request) error {
	b, err := base64.DecodeString("AQID")
	if err != nil { return err }
	data, ok := b.([]byte)
	if ok && len(data) == 3 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Ok = %v", got)
	}
}

func TestCompositeLiterals(t *testing.T) {
	t.Run("byte_slice_literal", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	b := []byte{72, 101, 108, 108, 111}
	req.Body = b
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if string(req.Body) != "Hello" {
			t.Fatalf("Body = %q", req.Body)
		}
	})

	t.Run("any_slice_literal", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sl := []any{"a", "b", "c"}
	req.SetHeader("X-Len", strconv.Itoa(len(sl)))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "3" {
			t.Fatalf("X-Len = %v", got)
		}
	})

	t.Run("map_literal", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	m := map[string]any{"key": "value"}
	v, ok := m["key"].(string)
	if ok {
		req.SetHeader("X-V", v)
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "value" {
			t.Fatalf("X-V = %v", got)
		}
	})
}

func TestMake(t *testing.T) {
	t.Run("make_byte_slice", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	b := make([]byte, 10)
	req.SetHeader("X-Len", strconv.Itoa(len(b)))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "10" {
			t.Fatalf("X-Len = %v", got)
		}
	})

	t.Run("make_any_slice", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sl := make([]any, 5)
	req.SetHeader("X-Len", strconv.Itoa(len(sl)))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "5" {
			t.Fatalf("X-Len = %v", got)
		}
	})

	t.Run("make_map", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	m := make(map[string]any)
	m["a"] = "1"
	req.SetHeader("X-Len", strconv.Itoa(len(m)))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "1" {
			t.Fatalf("X-Len = %v", got)
		}
	})
}

func TestSetFieldAndSetIndex(t *testing.T) {
	t.Run("set_request_method", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.Method = "POST"
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if req.Method != "POST" {
			t.Fatalf("Method = %q", req.Method)
		}
	})

	t.Run("set_request_raw_query", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.RawQuery = "x=1"
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if req.RawQuery != "x=1" {
			t.Fatalf("RawQuery = %q", req.RawQuery)
		}
	})

	t.Run("set_request_headers", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	h := req.Headers
	h["X-Custom"] = []any{"val"}
	req.Headers = h
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Custom"]; len(got) != 1 || got[0] != "val" {
			t.Fatalf("X-Custom = %v", got)
		}
	})

	t.Run("set_response_status", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.Status = 404
	return nil
}`
		resp := &Response{Status: 200}
		mustRun(t, "response", src, defaultReq(), resp)
		if resp.Status != 404 {
			t.Fatalf("Status = %d, want 404", resp.Status)
		}
	})

	t.Run("set_response_body", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.Body = []byte("modified")
	return nil
}`
		resp := &Response{Status: 200, Body: []byte("original")}
		mustRun(t, "response", src, defaultReq(), resp)
		if string(resp.Body) != "modified" {
			t.Fatalf("Body = %q", resp.Body)
		}
	})

	t.Run("set_response_headers", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	h := resp.Headers
	h["X-Out"] = []any{"val"}
	resp.Headers = h
	return nil
}`
		resp := &Response{Status: 200, Headers: map[string][]string{}}
		mustRun(t, "response", src, defaultReq(), resp)
		if got := resp.Headers["X-Out"]; len(got) != 1 || got[0] != "val" {
			t.Fatalf("X-Out = %v", got)
		}
	})

	t.Run("set_index_map_string_any", func(t *testing.T) {
		src := `package hook
import "encoding/json"
func OnRequest(req *Request) error {
	v, err := json.Decode(req.Body)
	if err != nil { return err }
	m, _ := v.(map[string]any)
	m["new"] = "added"
	out, err := json.Marshal(m)
	if err != nil { return err }
	req.Body = out
	return nil
}`
		req := defaultReq()
		req.Body = []byte(`{"old":"val"}`)
		mustRun(t, "request", src, req, nil)
		if !strings.Contains(string(req.Body), `"new":"added"`) {
			t.Fatalf("Body = %s", req.Body)
		}
	})

	t.Run("set_index_byte_slice", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	b := req.Body
	b[0] = 65
	return nil
}`
		req := defaultReq()
		req.Body = []byte("hello")
		mustRun(t, "request", src, req, nil)
		if req.Body[0] != 65 {
			t.Fatalf("Body[0] = %d", req.Body[0])
		}
	})

	t.Run("set_index_any_slice", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	sl := make([]any, 3)
	sl[0] = "first"
	sl[1] = "second"
	sl[2] = "third"
	req.SetHeader("X-V", sl[1].(string))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "second" {
			t.Fatalf("X-V = %v", got)
		}
	})

	t.Run("set_index_header_map", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.Headers["X-Direct"] = []any{"yes"}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Direct"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Direct = %v", got)
		}
	})
}

func TestGetField(t *testing.T) {
	t.Run("request_all_fields", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.SetHeader("X-Scheme", req.Scheme)
	req.SetHeader("X-Host", req.Host)
	req.SetHeader("X-Port", req.Port)
	req.SetHeader("X-Method", req.Method)
	req.SetHeader("X-Path", req.Path)
	req.SetHeader("X-Query", req.RawQuery)
	return nil
}`
		req := &Request{
			Scheme: "https", Host: "api.test", Port: "8080",
			Method: "PUT", Path: "/update", RawQuery: "id=5",
			Headers: map[string][]string{},
		}
		mustRun(t, "request", src, req, nil)
		checks := map[string]string{
			"X-Scheme": "https", "X-Host": "api.test", "X-Port": "8080",
			"X-Method": "PUT", "X-Path": "/update", "X-Query": "id=5",
		}
		for k, want := range checks {
			if got := req.Headers[k]; len(got) != 1 || got[0] != want {
				t.Errorf("%s = %v, want %q", k, got, want)
			}
		}
	})

	t.Run("response_all_fields", func(t *testing.T) {
		src := `package hook
import "strconv"
func OnResponse(req *Request, resp *Response) error {
	resp.SetHeader("X-Status", strconv.Itoa(resp.Status))
	resp.SetHeader("X-Bodylen", strconv.Itoa(len(resp.Body)))
	return nil
}`
		resp := &Response{Status: 201, Body: []byte("created"), Headers: map[string][]string{}}
		mustRun(t, "response", src, defaultReq(), resp)
		if got := resp.Headers["X-Status"]; len(got) != 1 || got[0] != "201" {
			t.Fatalf("X-Status = %v", got)
		}
		if got := resp.Headers["X-Bodylen"]; len(got) != 1 || got[0] != "7" {
			t.Fatalf("X-Bodylen = %v", got)
		}
	})
}

func TestCallMethod(t *testing.T) {
	t.Run("add_header", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.AddHeader("X-Multi", "a")
	req.AddHeader("X-Multi", "b")
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Multi"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Fatalf("X-Multi = %v", got)
		}
	})

	t.Run("response_set_del_header", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.SetHeader("X-Added", "1")
	resp.SetHeader("X-Kill", "temp")
	resp.DelHeader("X-Kill")
	return nil
}`
		resp := &Response{Status: 200, Headers: map[string][]string{}}
		mustRun(t, "response", src, defaultReq(), resp)
		if got := resp.Headers["X-Added"]; len(got) != 1 || got[0] != "1" {
			t.Fatalf("X-Added = %v", got)
		}
		if _, exists := resp.Headers["X-Kill"]; exists {
			t.Fatal("X-Kill should have been deleted")
		}
	})

	t.Run("get_header", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	v := req.GetHeader("X-In")
	req.SetHeader("X-Got", v)
	miss := req.GetHeader("X-Nothing")
	req.SetHeader("X-Miss", miss)
	return nil
}`
		req := defaultReq()
		req.Headers["X-In"] = []string{"hello"}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Got"]; len(got) != 1 || got[0] != "hello" {
			t.Fatalf("X-Got = %v", got)
		}
		if got := req.Headers["X-Miss"]; len(got) != 1 || got[0] != "" {
			t.Fatalf("X-Miss = %v", got)
		}
	})

	t.Run("response_get_header", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	v := resp.GetHeader("Content-Type")
	resp.SetHeader("X-Got", v)
	return nil
}`
		resp := &Response{Status: 200, Headers: map[string][]string{"Content-Type": {"application/json"}}}
		mustRun(t, "response", src, defaultReq(), resp)
		if got := resp.Headers["X-Got"]; len(got) != 1 || got[0] != "application/json" {
			t.Fatalf("X-Got = %v", got)
		}
	})

	t.Run("error_method", func(t *testing.T) {
		src := `package hook
import "errors"
func OnRequest(req *Request) error {
	e := errors.New("test error")
	req.SetHeader("X-Err", e.Error())
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Err"]; len(got) != 1 || got[0] != "test error" {
			t.Fatalf("X-Err = %v", got)
		}
	})
}

func TestStdlibBytes(t *testing.T) {
	src := `package hook
import "bytes"
func OnRequest(req *Request) error {
	b := req.Body
	if bytes.HasPrefix(b, "he") {
		req.SetHeader("X-Pre", "yes")
	}
	if bytes.HasSuffix(b, "lo") {
		req.SetHeader("X-Suf", "yes")
	}
	if bytes.Equal(b, "hello") {
		req.SetHeader("X-Eq", "yes")
	}
	idx := bytes.Index(b, "ll")
	if idx == 2 {
		req.SetHeader("X-Idx", "yes")
	}
	low := bytes.ToLower([]byte("ABC"))
	req.SetHeader("X-Low", string(low))
	up := bytes.ToUpper([]byte("abc"))
	req.SetHeader("X-Up", string(up))
	trimmed := bytes.TrimSpace([]byte("  hi  "))
	req.SetHeader("X-Trim", string(trimmed))
	replaced := bytes.ReplaceAll(b, "l", "r")
	req.SetHeader("X-Rep", string(replaced))
	rep := bytes.Repeat([]byte("ab"), 3)
	req.SetHeader("X-Rpt", string(rep))
	return nil
}`
	req := defaultReq()
	req.Body = []byte("hello")
	mustRun(t, "request", src, req, nil)
	checks := map[string]string{
		"X-Pre": "yes", "X-Suf": "yes", "X-Eq": "yes", "X-Idx": "yes",
		"X-Low": "abc", "X-Up": "ABC", "X-Trim": "hi", "X-Rep": "herro",
		"X-Rpt": "ababab",
	}
	for k, want := range checks {
		if got := req.Headers[k]; len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want %q", k, got, want)
		}
	}
}

func TestStdlibStrings(t *testing.T) {
	src := `package hook
import (
	"strings"
	"strconv"
)
func OnRequest(req *Request) error {
	req.SetHeader("X-Idx", strconv.Itoa(strings.Index("hello world", "world")))
	req.SetHeader("X-Trimprefix", strings.TrimPrefix("v1/api", "v1/"))
	req.SetHeader("X-Trimsuffix", strings.TrimSuffix("file.txt", ".txt"))
	req.SetHeader("X-Low", strings.ToLower("HELLO"))
	req.SetHeader("X-Up", strings.ToUpper("hello"))
	req.SetHeader("X-Trim", strings.TrimSpace("  hi  "))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	checks := map[string]string{
		"X-Idx": "6", "X-Trimprefix": "api", "X-Trimsuffix": "file",
		"X-Low": "hello", "X-Up": "HELLO", "X-Trim": "hi",
	}
	for k, want := range checks {
		if got := req.Headers[k]; len(got) != 1 || got[0] != want {
			t.Errorf("%s = %v, want %q", k, got, want)
		}
	}
}

func TestStdlibHex(t *testing.T) {
	src := `package hook
import "encoding/hex"
func OnRequest(req *Request) error {
	encoded := hex.EncodeToString([]byte{0xDE, 0xAD})
	req.SetHeader("X-Enc", encoded)
	decoded, err := hex.DecodeString("CAFE")
	if err != nil { return err }
	if len(decoded) == 2 {
		req.SetHeader("X-Dec", "ok")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Enc"]; len(got) != 1 || got[0] != "dead" {
		t.Fatalf("X-Enc = %v", got)
	}
	if got := req.Headers["X-Dec"]; len(got) != 1 || got[0] != "ok" {
		t.Fatalf("X-Dec = %v", got)
	}
}

func TestStdlibBase64(t *testing.T) {
	src := `package hook
import "encoding/base64"
func OnRequest(req *Request) error {
	encoded := base64.EncodeToString([]byte("hello"))
	req.SetHeader("X-Enc", encoded)
	decoded, err := base64.DecodeString(encoded)
	if err != nil { return err }
	req.SetHeader("X-Dec", string(decoded))
	urlEnc := base64.URLEncodeToString([]byte("hello?world"))
	req.SetHeader("X-Urlenc", urlEnc)
	urlDec, err := base64.URLDecodeString(urlEnc)
	if err != nil { return err }
	req.SetHeader("X-Urldec", string(urlDec))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Enc"]; len(got) != 1 || got[0] != "aGVsbG8=" {
		t.Fatalf("X-Enc = %v", got)
	}
	if got := req.Headers["X-Dec"]; len(got) != 1 || got[0] != "hello" {
		t.Fatalf("X-Dec = %v", got)
	}
}

func TestStdlibStrconv(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	n, err := strconv.Atoi("42")
	if err != nil { return err }
	req.SetHeader("X-Atoi", strconv.Itoa(n))

	b, err := strconv.ParseBool("true")
	if err != nil { return err }
	req.SetHeader("X-Parsed", strconv.FormatBool(b))

	req.SetHeader("X-Formatted", strconv.FormatBool(false))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Atoi"]; len(got) != 1 || got[0] != "42" {
		t.Fatalf("X-Atoi = %v", got)
	}
	if got := req.Headers["X-Parsed"]; len(got) != 1 || got[0] != "true" {
		t.Fatalf("X-Parsed = %v", got)
	}
	if got := req.Headers["X-Formatted"]; len(got) != 1 || got[0] != "false" {
		t.Fatalf("X-Formatted = %v", got)
	}
}

func TestStdlibErrors(t *testing.T) {
	src := `package hook
import "errors"
func OnRequest(req *Request) error {
	e := errors.New("custom error")
	return e
}`
	req := defaultReq()
	err := compileAndRun(t, "request", src, req, nil)
	if err == nil || err.Error() != "custom error" {
		t.Fatalf("err = %v, want 'custom error'", err)
	}
}

func TestStdlibSHA256(t *testing.T) {
	src := `package hook
import "crypto/sha256"
func OnRequest(req *Request) error {
	sum := sha256.Sum256([]byte("test"))
	if len(sum) == 32 {
		req.SetHeader("X-Len", "32")
	}
	hexSum := sha256.Sum256Hex([]byte("test"))
	if len(hexSum) == 64 {
		req.SetHeader("X-Hex", "64")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "32" {
		t.Fatalf("X-Len = %v", got)
	}
	if got := req.Headers["X-Hex"]; len(got) != 1 || got[0] != "64" {
		t.Fatalf("X-Hex = %v", got)
	}
}

func TestStdlibRegexp(t *testing.T) {
	t.Run("find_string", func(t *testing.T) {
		src := `package hook
import "regexp"
func OnRequest(req *Request) error {
	found := regexp.FindString("[0-9]+", "abc123def")
	req.SetHeader("X-Found", found)
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Found"]; len(got) != 1 || got[0] != "123" {
			t.Fatalf("X-Found = %v", got)
		}
	})

	t.Run("replace_all_string", func(t *testing.T) {
		src := `package hook
import "regexp"
func OnRequest(req *Request) error {
	result := regexp.ReplaceAllString("[0-9]+", "abc123def456", "NUM")
	req.SetHeader("X-Rep", result)
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Rep"]; len(got) != 1 || got[0] != "abcNUMdefNUM" {
			t.Fatalf("X-Rep = %v", got)
		}
	})
}

func TestStdlibURL(t *testing.T) {
	src := `package hook
import "net/url"
func OnRequest(req *Request) error {
	escaped := url.QueryEscape("hello world&foo=bar")
	req.SetHeader("X-Esc", escaped)

	unescaped, err := url.QueryUnescape("hello+world%26foo%3Dbar")
	if err != nil { return err }
	req.SetHeader("X-Unesc", unescaped)

	vals, err := url.ParseQuery("a=1&b=2&a=3")
	if err != nil { return err }
	encoded := url.EncodeQuery(vals)
	req.SetHeader("X-Enc", encoded)
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Esc"]; len(got) != 1 || got[0] != "hello+world%26foo%3Dbar" {
		t.Fatalf("X-Esc = %v", got)
	}
	if got := req.Headers["X-Unesc"]; len(got) != 1 || got[0] != "hello world&foo=bar" {
		t.Fatalf("X-Unesc = %v", got)
	}
}

func TestStdlibHMAC(t *testing.T) {
	src := `package hook
import "crypto/hmac"
func OnRequest(req *Request) error {
	sum := hmac.SHA256([]byte("key"), []byte("data"))
	if len(sum) == 32 {
		req.SetHeader("X-Len", "32")
	}
	hexSum := hmac.SHA256Hex([]byte("key"), []byte("data"))
	if len(hexSum) == 64 {
		req.SetHeader("X-Hex", "64")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "32" {
		t.Fatalf("X-Len = %v", got)
	}
	if got := req.Headers["X-Hex"]; len(got) != 1 || got[0] != "64" {
		t.Fatalf("X-Hex = %v", got)
	}
}

func TestConstDeclarations(t *testing.T) {
	t.Run("const_bool_and_negative", func(t *testing.T) {
		src := `package hook
import "strconv"
const (
	flag   = true
	offset = -10
	rate   = 3.14
	negF   = -2.5
)
func OnRequest(req *Request) error {
	if flag {
		req.SetHeader("X-Flag", "true")
	}
	req.SetHeader("X-Off", strconv.Itoa(offset))
	if rate > 3.0 {
		req.SetHeader("X-Rate", "yes")
	}
	if negF < 0.0 {
		req.SetHeader("X-Negfloat", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		checks := map[string]string{
			"X-Flag": "true", "X-Off": "-10", "X-Rate": "yes", "X-Negfloat": "yes",
		}
		for k, want := range checks {
			if got := req.Headers[k]; len(got) != 1 || got[0] != want {
				t.Errorf("%s = %v, want %q", k, got, want)
			}
		}
	})

	t.Run("const_char_literal", func(t *testing.T) {
		src := `package hook
import "strconv"
const ch = 'A'
func OnRequest(req *Request) error {
	req.SetHeader("X-Ch", strconv.Itoa(ch))
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ch"]; len(got) != 1 || got[0] != "65" {
			t.Fatalf("X-Ch = %v", got)
		}
	})
}

func TestUserDefinedFunctions(t *testing.T) {
	src := `package hook
func add(a int, b int) int {
	return a + b
}
func OnRequest(req *Request) error {
	sum := add(10, 20)
	if sum == 30 {
		req.SetHeader("X-Sum", "30")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Sum"]; len(got) != 1 || got[0] != "30" {
		t.Fatalf("X-Sum = %v", got)
	}
}

func TestCallDepthLimitCoverage(t *testing.T) {
	src := `package hook
func recurse(n int) int {
	if n == 0 {
		return 0
	}
	return recurse(n - 1) + 1
}
func OnRequest(req *Request) error {
	_ = recurse(100)
	return nil
}`
	prog, err := Compile("request", src)
	if err != nil {
		t.Fatal(err)
	}
	err = prog.RunRequest(context.Background(), defaultReq(), RunOptions{})
	if !errors.Is(err, ErrCallDepth) {
		t.Fatalf("err = %v, want ErrCallDepth", err)
	}
}

func TestPhaseErrors(t *testing.T) {
	t.Run("request_hook_as_response", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error { return nil }`
		prog, err := Compile("request", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunResponse(context.Background(), defaultReq(), &Response{Status: 200}, RunOptions{})
		if err == nil || !strings.Contains(err.Error(), "cannot run") {
			t.Fatalf("err = %v, want phase mismatch", err)
		}
	})

	t.Run("response_hook_as_request", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error { return nil }`
		prog, err := Compile("response", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunRequest(context.Background(), defaultReq(), RunOptions{})
		if err == nil || !strings.Contains(err.Error(), "cannot run") {
			t.Fatalf("err = %v, want phase mismatch", err)
		}
	})
}

func TestCompileValidation(t *testing.T) {
	t.Run("bad_phase", func(t *testing.T) {
		_, err := Compile("invalid", `package hook; func OnRequest(req *Request) error { return nil }`)
		if err == nil || !strings.Contains(err.Error(), "invalid hook phase") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("empty_source", func(t *testing.T) {
		_, err := Compile("request", "")
		if err == nil {
			t.Fatal("expected error for empty source")
		}
	})

	t.Run("custom_type", func(t *testing.T) {
		_, err := Compile("request", `package hook; type Foo struct{}; func OnRequest(req *Request) error { return nil }`)
		if err == nil || !strings.Contains(err.Error(), "custom package-level types") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("receiver_method", func(t *testing.T) {
		_, err := Compile("request", `package hook; type Foo struct{}; func (f Foo) Bar() {}; func OnRequest(req *Request) error { return nil }`)
		if err == nil {
			t.Fatal("expected error for receiver method or custom type")
		}
	})

	t.Run("import_alias", func(t *testing.T) {
		src := `package hook
import s "strings"
func OnRequest(req *Request) error {
	if s.Contains("hello", "ell") {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
		req := defaultReq()
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("wrong_return_count", func(t *testing.T) {
		_, err := Compile("request", `package hook; func OnRequest(req *Request) {}`)
		if err == nil || !strings.Contains(err.Error(), "return error") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("parse_error", func(t *testing.T) {
		_, err := Compile("request", `package hook; func OnRequest(req *Request error { return nil }`)
		if err == nil || !strings.Contains(err.Error(), "parse hook") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestIfElse(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	x := 5
	if x > 10 {
		req.SetHeader("X-R", "gt10")
	} else if x > 3 {
		req.SetHeader("X-R", "gt3")
	} else {
		req.SetHeader("X-R", "other")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-R"]; len(got) != 1 || got[0] != "gt3" {
		t.Fatalf("X-R = %v, want gt3", got)
	}
}

func TestIfInit(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	if n, err := strconv.Atoi("42"); err == nil {
		req.SetHeader("X-N", strconv.Itoa(n))
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-N"]; len(got) != 1 || got[0] != "42" {
		t.Fatalf("X-N = %v", got)
	}
}

func TestByteConversion(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	b := []byte(req.Path)
	req.Body = b
	s := string(req.Body)
	req.SetHeader("X-S", s)
	return nil
}`
	req := defaultReq()
	req.Path = "/converted"
	mustRun(t, "request", src, req, nil)
	if string(req.Body) != "/converted" {
		t.Fatalf("Body = %q", req.Body)
	}
	if got := req.Headers["X-S"]; len(got) != 1 || got[0] != "/converted" {
		t.Fatalf("X-S = %v", got)
	}
}

func TestReadOnlyMutationPaths(t *testing.T) {
	t.Run("set_field_in_response_phase", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	req.Method = "EVIL"
	return nil
}`
		prog, err := Compile("response", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunResponse(context.Background(), defaultReq(), &Response{Status: 200}, RunOptions{})
		if !errors.Is(err, ErrForbiddenMutation) {
			t.Fatalf("err = %v, want ErrForbiddenMutation", err)
		}
	})

	t.Run("del_header_in_response_phase", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	req.DelHeader("X-Test")
	return nil
}`
		prog, err := Compile("response", src)
		if err != nil {
			t.Fatal(err)
		}
		err = prog.RunResponse(context.Background(), defaultReq(), &Response{Status: 200}, RunOptions{})
		if !errors.Is(err, ErrForbiddenMutation) {
			t.Fatalf("err = %v, want ErrForbiddenMutation", err)
		}
	})
}

func TestUnknownFieldErrors(t *testing.T) {
	t.Run("unknown_request_field_set", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.Unknown = "x"
	return nil
}`
		err := compileAndRun(t, "request", src, defaultReq(), nil)
		if err == nil || !strings.Contains(err.Error(), "unknown Request field") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unknown_response_field_set", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.Unknown = "x"
	return nil
}`
		err := compileAndRun(t, "response", src, defaultReq(), &Response{Status: 200})
		if err == nil || !strings.Contains(err.Error(), "unknown Response field") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("unknown_field_get", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	_ = req.Fake
	return nil
}`
		err := compileAndRun(t, "request", src, defaultReq(), nil)
		if err == nil || !strings.Contains(err.Error(), "unknown field") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestNilHeadersInit(t *testing.T) {
	t.Run("request_nil_headers", func(t *testing.T) {
		src := `package hook
func OnRequest(req *Request) error {
	req.SetHeader("X-Ok", "yes")
	return nil
}`
		req := &Request{Scheme: "https", Host: "test", Port: "443"}
		mustRun(t, "request", src, req, nil)
		if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})

	t.Run("response_nil_headers", func(t *testing.T) {
		src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.SetHeader("X-Ok", "yes")
	return nil
}`
		resp := &Response{Status: 200}
		mustRun(t, "response", src, &Request{}, resp)
		if got := resp.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
			t.Fatalf("X-Ok = %v", got)
		}
	})
}

func TestReturnNonError(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
}

func TestMultiReturnValue(t *testing.T) {
	src := `package hook
import "strconv"
func pair() (int, int) {
	return 1, 2
}
func OnRequest(req *Request) error {
	a, b := pair()
	req.SetHeader("X-Sum", strconv.Itoa(a + b))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Sum"]; len(got) != 1 || got[0] != "3" {
		t.Fatalf("X-Sum = %v", got)
	}
}

func TestPanicError(t *testing.T) {
	pe := &PanicError{Value: "test"}
	if pe.Error() != "hook panic: test" {
		t.Fatalf("PanicError.Error() = %q", pe.Error())
	}
}

func TestSliceBoundsError(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	s := "abc"
	_ = s[0:10]
	return nil
}`
	prog, err := Compile("request", src)
	if err != nil {
		t.Fatal(err)
	}
	err = prog.RunRequest(context.Background(), defaultReq(), RunOptions{})
	if err == nil || !strings.Contains(err.Error(), "slice bounds") {
		t.Fatalf("err = %v, want slice bounds error", err)
	}
}

func TestIndexOutOfRange(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	b := req.Body
	_ = b[100]
	return nil
}`
	prog, err := Compile("request", src)
	if err != nil {
		t.Fatal(err)
	}
	err = prog.RunRequest(context.Background(), &Request{Body: []byte("hi")}, RunOptions{})
	if err == nil || !strings.Contains(err.Error(), "index out of range") {
		t.Fatalf("err = %v, want index out of range", err)
	}
}

func TestSetFieldOnNonStruct(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	s := "hello"
	s.X = "y"
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot set field") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestHeadersBadType(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	req.Headers = "bad"
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "map[string][]string") {
		t.Fatalf("err = %v", err)
	}
}

func TestResponseHeadersBadType(t *testing.T) {
	src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.Headers = "bad"
	return nil
}`
	err := compileAndRun(t, "response", src, defaultReq(), &Response{Status: 200})
	if err == nil || !strings.Contains(err.Error(), "map[string][]string") {
		t.Fatalf("err = %v", err)
	}
}

func TestCannotIndexType(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	x := 42
	_ = x[0]
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot index") {
		t.Fatalf("err = %v", err)
	}
}

func TestCannotSliceType(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	x := 42
	_ = x[0:1]
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil {
		t.Fatal("expected error for slicing int")
	}
}

func TestCannotRangeType(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	x := 42
	for _, _ = range x {
	}
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot range") {
		t.Fatalf("err = %v", err)
	}
}

func TestCannotIndexAssignType(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	x := "hello"
	x[0] = 65
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "cannot index-assign") {
		t.Fatalf("err = %v", err)
	}
}

func TestUndefinedVariable(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	x = 42
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "undefined variable") {
		t.Fatalf("err = %v", err)
	}
}

func TestUndefinedFunction(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	_ = notExist()
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "undefined function") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnsupportedStdlibCall(t *testing.T) {
	src := `package hook
import "strings"
func OnRequest(req *Request) error {
	_ = strings.NewReader("x")
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported stdlib call") {
		t.Fatalf("err = %v", err)
	}
}

func TestMakeNegativeLength(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	_ = make([]byte, -1)
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "negative length") {
		t.Fatalf("err = %v", err)
	}
}

func TestBlockStmt(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	x := 1
	{
		x = 2
	}
	req.SetHeader("X-V", strconv.Itoa(x))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "2" {
		t.Fatalf("X-V = %v", got)
	}
}

func TestEmptyReturn(t *testing.T) {
	src := `package hook
func helper() {
	return
}
func OnRequest(req *Request) error {
	helper()
	req.SetHeader("X-Ok", "yes")
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Ok = %v", got)
	}
}

func TestStringSliceIndex(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	vals := req.Headers["X-In"]
	if len(vals) > 0 {
		req.SetHeader("X-V", vals[0])
	}
	return nil
}`
	req := defaultReq()
	req.Headers["X-In"] = []string{"first", "second"}
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "first" {
		t.Fatalf("X-V = %v", got)
	}
}

func TestStringSliceSlice(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	vals := req.Headers["X-In"]
	sub := vals[1:]
	req.SetHeader("X-Len", strconv.Itoa(len(sub)))
	return nil
}`
	req := defaultReq()
	req.Headers["X-In"] = []string{"a", "b", "c"}
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "2" {
		t.Fatalf("X-Len = %v", got)
	}
}

func TestFloatLiterals(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	f := 3.14
	if f > 3.0 && f < 4.0 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Ok = %v", got)
	}
}

func TestMixedIntFloat(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	n := 10
	f := 2.5
	result := f + float64(n)
	if result == 12.5 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Ok = %v", got)
	}
}

func TestAssignCountMismatch(t *testing.T) {
	src := `package hook
func pair() (int, int) { return 1, 2 }
func OnRequest(req *Request) error {
	a, b, c := pair()
	_ = a
	_ = b
	_ = c
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "assignment count mismatch") {
		t.Fatalf("err = %v", err)
	}
}

func TestRangeWithAssign(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	sum := 0
	items := []any{10, 20, 30}
	k := 0
	v := 0
	for k, v = range items {
		sum += v
	}
	req.SetHeader("X-Sum", strconv.Itoa(sum))
	req.SetHeader("X-K", strconv.Itoa(k))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Sum"]; len(got) != 1 || got[0] != "60" {
		t.Fatalf("X-Sum = %v", got)
	}
	if got := req.Headers["X-K"]; len(got) != 1 || got[0] != "2" {
		t.Fatalf("X-K = %v", got)
	}
}

func TestResponseAddHeader(t *testing.T) {
	src := `package hook
func OnResponse(req *Request, resp *Response) error {
	resp.AddHeader("X-Multi", "a")
	resp.AddHeader("X-Multi", "b")
	return nil
}`
	resp := &Response{Status: 200, Headers: map[string][]string{}}
	mustRun(t, "response", src, defaultReq(), resp)
	if got := resp.Headers["X-Multi"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("X-Multi = %v", got)
	}
}

func TestResponseGetHeader(t *testing.T) {
	src := `package hook
func OnResponse(req *Request, resp *Response) error {
	v := req.GetHeader("X-In")
	resp.SetHeader("X-Got", v)
	return nil
}`
	req := defaultReq()
	req.Headers["X-In"] = []string{"hello"}
	resp := &Response{Status: 200, Headers: map[string][]string{}}
	mustRun(t, "response", src, req, resp)
	if got := resp.Headers["X-Got"]; len(got) != 1 || got[0] != "hello" {
		t.Fatalf("X-Got = %v", got)
	}
}

func TestRangeReturnFromRange(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	items := []any{"a", "b", "stop", "d"}
	for _, v := range items {
		s, _ := v.(string)
		if s == "stop" {
			req.SetHeader("X-V", s)
			return nil
		}
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "stop" {
		t.Fatalf("X-V = %v", got)
	}
}

func TestVarDeclWithInit(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	var x int = 42
	if x == 42 {
		req.SetHeader("X-Ok", "yes")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Ok"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Ok = %v", got)
	}
}

func TestParenExpr(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	x := (2 + 3) * 4
	req.SetHeader("X-V", strconv.Itoa(x))
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "20" {
		t.Fatalf("X-V = %v", got)
	}
}

func TestTypeAssertInt64(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	var v any
	v = 42
	n, ok := v.(int)
	if ok {
		req.SetHeader("X-V", strconv.Itoa(n))
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-V"]; len(got) != 1 || got[0] != "42" {
		t.Fatalf("X-V = %v", got)
	}
}

func TestValuesEqualIntTypes(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	var a any
	var b any
	a = 42
	b = 42
	if a == b {
		req.SetHeader("X-Eq", "yes")
	}
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Eq"]; len(got) != 1 || got[0] != "yes" {
		t.Fatalf("X-Eq = %v", got)
	}
}

func TestAssignTargetToSelector(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	req.Path = "/new"
	req.Body = []byte("body")
	return nil
}`
	req := defaultReq()
	mustRun(t, "request", src, req, nil)
	if req.Path != "/new" {
		t.Fatalf("Path = %q", req.Path)
	}
	if string(req.Body) != "body" {
		t.Fatalf("Body = %q", req.Body)
	}
}

func TestLenStringSlice(t *testing.T) {
	src := `package hook
import "strconv"
func OnRequest(req *Request) error {
	vals := req.Headers["X-In"]
	req.SetHeader("X-Len", strconv.Itoa(len(vals)))
	return nil
}`
	req := defaultReq()
	req.Headers["X-In"] = []string{"a", "b", "c"}
	mustRun(t, "request", src, req, nil)
	if got := req.Headers["X-Len"]; len(got) != 1 || got[0] != "3" {
		t.Fatalf("X-Len = %v", got)
	}
}

func TestUndefinedIdentifier(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	_ = unknownVar
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "undefined identifier") {
		t.Fatalf("err = %v", err)
	}
}

func TestUnsupportedMethodOnType(t *testing.T) {
	src := `package hook
func OnRequest(req *Request) error {
	req.BadMethod()
	return nil
}`
	err := compileAndRun(t, "request", src, defaultReq(), nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported method") {
		t.Fatalf("err = %v", err)
	}
}
