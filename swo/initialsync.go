package swo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/target/goalert/swo/swogrp"
)

func (m *Manager) InitialSync(ctx context.Context, tables []Table, oldConn, newConn *pgx.Conn) error {
	srcTx, err := oldConn.BeginTx(ctx, pgx.TxOptions{
		AccessMode:     pgx.ReadOnly,
		IsoLevel:       pgx.Serializable,
		DeferrableMode: pgx.Deferrable,
	})
	if err != nil {
		return fmt.Errorf("begin src tx: %w", err)
	}
	defer srcTx.Rollback(ctx)

	dstTx, err := newConn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin dst tx: %w", err)
	}
	defer dstTx.Rollback(ctx)

	// defer all constraints
	_, err = dstTx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED")
	if err != nil {
		return fmt.Errorf("defer constraints: %w", err)
	}

	for _, table := range tables {
		if table.SkipSync() {
			continue
		}

		err = m.SyncTableInit(ctx, table, srcTx, dstTx)
		if err != nil {
			return fmt.Errorf("sync table %s: %w", table.Name, err)
		}
	}

	swogrp.Progressf(ctx, "commit initial sync")
	// Important to validate src commit, even though it's read-only.
	//
	// A failure here indicates the isolation level has been violated
	// and we will need to try again.
	err = srcTx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit src tx: %w", err)
	}

	err = dstTx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit dst tx: %w", err)
	}

	// vacuum analyze new DB
	swogrp.Progressf(ctx, "vacuum analyze")
	_, err = newConn.Exec(ctx, "vacuum analyze")
	if err != nil {
		return fmt.Errorf("vacuum analyze: %w", err)
	}

	return nil
}

type lineCount struct {
	n  int
	mx sync.Mutex
}

func (lc *lineCount) Write(p []byte) (n int, err error) {
	lc.mx.Lock()
	lc.n += bytes.Count(p, []byte("\n"))
	lc.mx.Unlock()
	return len(p), nil
}

func (lc *lineCount) Lines() int {
	lc.mx.Lock()
	defer lc.mx.Unlock()
	return lc.n
}

func (m *Manager) SyncTableInit(origCtx context.Context, t Table, srcTx, dstTx pgx.Tx) error {
	ctx, cancel := context.WithCancel(origCtx)
	defer cancel()

	var rowCount int
	err := srcTx.QueryRow(ctx, fmt.Sprintf("select count(*) from %s", t.QuotedName())).Scan(&rowCount)
	if err != nil {
		return fmt.Errorf("count rows: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	pr, pw := io.Pipe()
	var lc lineCount
	go func() {
		defer wg.Done()
		prog := time.NewTimer(2 * time.Second)
		defer prog.Stop()
		for {
			swogrp.Progressf(origCtx, "syncing table %s (%d/%d)", t.Name, lc.Lines(), rowCount)
			select {
			case <-ctx.Done():
				pw.CloseWithError(ctx.Err())
				pr.CloseWithError(ctx.Err())
				return
			case <-prog.C:
			}
		}
	}()

	var srcErr, dstErr error

	go func() {
		defer wg.Done()
		_, srcErr = srcTx.Conn().PgConn().CopyTo(ctx, pw, fmt.Sprintf(`copy %s to stdout`, t.QuotedName()))
		pw.Close()
	}()
	go func() {
		defer wg.Done()
		defer cancel()
		_, dstErr = dstTx.Conn().PgConn().CopyFrom(ctx, io.TeeReader(pr, &lc), fmt.Sprintf(`copy %s from stdin`, t.QuotedName()))
		pr.Close()
	}()

	wg.Wait()

	if dstErr != nil {
		return fmt.Errorf("copy to dst: %w", dstErr)
	}
	if srcErr != nil {
		return fmt.Errorf("copy from src: %w", srcErr)
	}

	return nil
}