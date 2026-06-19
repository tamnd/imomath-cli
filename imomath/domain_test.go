package imomath

import (
	"testing"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "imomath" {
		t.Errorf("Scheme = %q, want imomath", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "imomath" {
		t.Errorf("Identity.Binary = %q, want imomath", info.Identity.Binary)
	}
}

func TestClassify_url(t *testing.T) {
	typ, id, err := Domain{}.Classify("https://www.imomath.com/index.cgi?p=algebra_problems1")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "problem" {
		t.Errorf("typ = %q, want problem", typ)
	}
	if id != "algebra_problems1" {
		t.Errorf("id = %q, want algebra_problems1", id)
	}
}

func TestClassify_id(t *testing.T) {
	typ, id, err := Domain{}.Classify("nt_basics")
	if err != nil {
		t.Fatal(err)
	}
	if typ != "problem" {
		t.Errorf("typ = %q, want problem", typ)
	}
	if id != "nt_basics" {
		t.Errorf("id = %q, want nt_basics", id)
	}
}

func TestLocate_problem(t *testing.T) {
	got, err := Domain{}.Locate("problem", "algebra_problems1")
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
	want := "https://www.imomath.com/index.cgi?p=algebra_problems1"
	if got != want {
		t.Errorf("Locate = %q, want %q", got, want)
	}
}

func TestLocate_unknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}
