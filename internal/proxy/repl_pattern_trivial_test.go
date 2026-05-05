package proxy

import "testing"

func TestIsTrivialShape_EmptyIsTrivial(t *testing.T) {
	if !isTrivialShape("") {
		t.Error("empty string should be trivial")
	}
}

func TestIsTrivialShape_SingleTokenIsTrivial(t *testing.T) {
	for _, s := range []string{"pwd", "whoami", "date", "make"} {
		if !isTrivialShape(s) {
			t.Errorf("%q (single token) should be trivial", s)
		}
	}
}

func TestIsTrivialShape_DenyListedFirstTokenIsTrivial(t *testing.T) {
	trivial := []string{
		"echo ?",
		"echo ? ?",
		"ls ?",
		"ls ? ?",
		"cat ?",
		"cat ? ? ?",
		"wc ?",
		"head ? ?",
		"tail ? ?",
		"env ?",
		"which ?",
	}
	for _, s := range trivial {
		if !isTrivialShape(s) {
			t.Errorf("%q should be trivial (deny-listed first token)", s)
		}
	}
}

func TestIsTrivialShape_GitCommandsTrivial(t *testing.T) {
	trivial := []string{
		"git add ?",
		"git commit ? ?",
		"git push ? ?",
		"git pull",
		"git status",
		"git diff ? ?",
		"git log ? ?",
		"git checkout ?",
		"git switch ?",
		"git branch ? ?",
		"git merge ?",
		"git rebase ?",
		"git stash",
		"git fetch",
		"git reset ? ?",
		"git tag ?",
		"git cherry-pick ?",
		"git remote ? ?",
	}
	for _, s := range trivial {
		if !isTrivialShape(s) {
			t.Errorf("%q should be trivial (git command)", s)
		}
	}
}

func TestIsTrivialShape_BashBuiltinsTrivial(t *testing.T) {
	trivial := []string{
		"mkdir ?",
		"mkdir ? ?",
		"rmdir ?",
		"rm ?",
		"rm ? ?",
		"cp ? ?",
		"mv ? ?",
		"touch ?",
		"chmod ? ?",
		"chown ? ?",
		"ln ? ? ?",
		"export ?",
		"source ?",
		"exit",
		"clear",
		"history",
		"whoami",
	}
	for _, s := range trivial {
		if !isTrivialShape(s) {
			t.Errorf("%q should be trivial (bash builtin)", s)
		}
	}
}

func TestIsTrivialShape_RealCommandsNotTrivial(t *testing.T) {
	nonTrivial := []string{
		"sqlite3 ? ?",
		"go build ?",
		"go test ? ? ? ?",
		"make deploy",
		"make test",
		"docker run ? ?",
		"curl ? ?",
		"sed ? ?",
		"awk ? ?",
		"grep ? ?",
		"rg ? ?",
	}
	for _, s := range nonTrivial {
		if isTrivialShape(s) {
			t.Errorf("%q should NOT be trivial (real command)", s)
		}
	}
}
