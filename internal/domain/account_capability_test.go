package domain

import "testing"

func TestCredentialMergePreservesOppositeSideAndRejectsStale(t *testing.T) {
	st := &AuthState{Cookie: "old", AccessToken: "open", CredentialGeneration: 4, OpenAuth: CredentialHealth{Status: AuthActive, VerifiedUID: "12"}}
	expected := uint64(4)
	h := CredentialHealth{Status: AuthActive, VerifiedUID: "12"}
	updated := MergeAuthCredentials(st, AuthCredentials{Cookie: "new", CookieAuth: &h, ExpectedGeneration: &expected})
	if updated.Cookie != "new" || updated.AccessToken != "open" || updated.OpenAuth != st.OpenAuth || updated.CredentialGeneration != 5 {
		t.Fatalf("incorrect merged state: %+v", updated)
	}
	stale := MergeAuthCredentials(updated, AuthCredentials{AccessToken: "stale", ExpectedGeneration: &expected})
	if stale.AccessToken != "open" || stale.CredentialGeneration != 5 {
		t.Fatal("stale update replaced credentials")
	}
}
