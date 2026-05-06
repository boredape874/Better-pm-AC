package meta

import "testing"

func TestAuthorityZeroIsValidate(t *testing.T) {
	var a Authority
	if a != AuthorityValidate {
		t.Fatalf("zero Authority=%v want %v", a, AuthorityValidate)
	}
}

func TestAuthorityStringStable(t *testing.T) {
	cases := map[Authority]string{
		AuthorityValidate:             "validate",
		AuthorityAuthoritativeMovement: "authoritative_movement",
		AuthorityAuthoritativeCombat:  "authoritative_combat",
	}
	for a, want := range cases {
		if a.String() != want {
			t.Fatalf("Authority(%d).String()=%q want %q", a, a.String(), want)
		}
	}
}
