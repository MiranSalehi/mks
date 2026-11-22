package mks

import (
	"errors"
	"testing"

	"github.com/goravel/framework/contracts/database/orm"
)

func TestAfterCommitRunsOnlyAfterACommit(t *testing.T) {
	p := &Panel{opts: Options{Orm: newSecurityTestORM(t)}}
	hc := &HookContext{}
	var ran []string

	err := p.transaction(hc, func(tx orm.Query) error {
		if hc.Query != tx {
			t.Fatal("hc.Query is the transaction")
		}
		hc.AfterCommit(func() { ran = append(ran, "committed") })
		if len(ran) != 0 {
			t.Fatal("inside the transaction the callback waits")
		}
		return nil
	})
	if err != nil || len(ran) != 1 {
		t.Fatalf("after a commit: ran %v, err %v", ran, err)
	}

	err = p.transaction(hc, func(orm.Query) error {
		hc.AfterCommit(func() { ran = append(ran, "rolled back") })
		return errors.New("fail")
	})
	if err == nil || len(ran) != 1 {
		t.Fatalf("after a rollback the callback must not run: %v", ran)
	}

	hc.AfterCommit(func() { ran = append(ran, "now") })
	if len(ran) != 2 {
		t.Fatal("outside a transaction it runs at once")
	}
}
