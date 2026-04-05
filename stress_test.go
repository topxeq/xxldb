// Package xxldb provides stress tests for concurrent access
package xxldb

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ==================== Basic Concurrency Tests ====================

// TestConcurrentInsert tests concurrent insert operations
func TestConcurrentInsert(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE concurrent_insert (id SEQ, value INT, thread_id INT)")
	if err != nil {
		t.Fatal(err)
	}

	const numGoroutines = 100
	const insertsPerGoroutine = 50

	var wg sync.WaitGroup
	var errors int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < insertsPerGoroutine; j++ {
				_, err := engine.Execute(fmt.Sprintf("INSERT INTO concurrent_insert (value, thread_id) VALUES (%d, %d)", j, threadID))
				if err != nil {
					atomic.AddInt64(&errors, 1)
					t.Errorf("Insert failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify all inserts succeeded
	result, err := engine.Execute("SELECT COUNT(*) FROM concurrent_insert")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	expected := int64(numGoroutines * insertsPerGoroutine)

	t.Logf("Concurrent Insert Results:")
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Inserts per goroutine: %d", insertsPerGoroutine)
	t.Logf("  Expected rows: %d", expected)
	t.Logf("  Actual rows: %d", count)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Throughput: %.2f inserts/sec", float64(expected)/elapsed.Seconds())

	if count != expected {
		t.Errorf("Data loss detected: expected %d rows, got %d", expected, count)
	}
	if errors > 0 {
		t.Errorf("Encountered %d errors during concurrent inserts", errors)
	}
}

// TestConcurrentRead tests concurrent read operations
func TestConcurrentRead(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Setup: insert test data
	_, err = engine.Execute("CREATE TABLE concurrent_read (id SEQ, value INT)")
	if err != nil {
		t.Fatal(err)
	}

	const numRows = 1000
	for i := 0; i < numRows; i++ {
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO concurrent_read (value) VALUES (%d)", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numReaders = 50
	const readsPerReader = 100

	var wg sync.WaitGroup
	var errors int64
	var totalReadTime int64

	start := time.Now()

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < readsPerReader; j++ {
				readStart := time.Now()
				result, err := engine.Execute("SELECT * FROM concurrent_read WHERE value >= 500")
				readTime := time.Since(readStart)
				atomic.AddInt64(&totalReadTime, int64(readTime))

				if err != nil {
					atomic.AddInt64(&errors, 1)
					t.Errorf("Read failed: %v", err)
				} else if len(result.Rows) < 500 {
					atomic.AddInt64(&errors, 1)
					t.Errorf("Unexpected row count: %d", len(result.Rows))
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)
	totalReads := int64(numReaders * readsPerReader)

	t.Logf("Concurrent Read Results:")
	t.Logf("  Readers: %d", numReaders)
	t.Logf("  Reads per reader: %d", readsPerReader)
	t.Logf("  Total reads: %d", totalReads)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Total duration: %v", elapsed)
	t.Logf("  Avg read time: %v", time.Duration(totalReadTime/totalReads))
	t.Logf("  Throughput: %.2f reads/sec", float64(totalReads)/elapsed.Seconds())

	if errors > 0 {
		t.Errorf("Encountered %d errors during concurrent reads", errors)
	}
}

// TestConcurrentReadWriteMix tests mixed concurrent read and write operations
func TestConcurrentReadWriteMix(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE concurrent_mix (id SEQ, value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Initial data
	for i := 0; i < 100; i++ {
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO concurrent_mix (value) VALUES (%d)", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numWriters = 20
	const numReaders = 30
	const opsPerThread = 50

	var wg sync.WaitGroup
	var readErrors int64
	var writeErrors int64

	start := time.Now()

	// Writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < opsPerThread; j++ {
				_, err := engine.Execute(fmt.Sprintf("INSERT INTO concurrent_mix (value) VALUES (%d)", threadID*1000+j))
				if err != nil {
					atomic.AddInt64(&writeErrors, 1)
				}
			}
		}(i)
	}

	// Readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerThread; j++ {
				_, err := engine.Execute("SELECT COUNT(*) FROM concurrent_mix")
				if err != nil {
					atomic.AddInt64(&readErrors, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify final count
	result, err := engine.Execute("SELECT COUNT(*) FROM concurrent_mix")
	if err != nil {
		t.Fatal(err)
	}
	finalCount, _ := result.Rows[0].Data[0].ToInt64()
	expectedMin := int64(100 + numWriters*opsPerThread)

	t.Logf("Concurrent Read/Write Mix Results:")
	t.Logf("  Writers: %d", numWriters)
	t.Logf("  Readers: %d", numReaders)
	t.Logf("  Ops per thread: %d", opsPerThread)
	t.Logf("  Read errors: %d", readErrors)
	t.Logf("  Write errors: %d", writeErrors)
	t.Logf("  Final row count: %d (min expected: %d)", finalCount, expectedMin)
	t.Logf("  Duration: %v", elapsed)

	if readErrors > 0 || writeErrors > 0 {
		t.Errorf("Encountered errors: %d read errors, %d write errors", readErrors, writeErrors)
	}
	if finalCount < expectedMin {
		t.Errorf("Data loss detected: expected at least %d rows, got %d", expectedMin, finalCount)
	}
}

// ==================== Update/Delete Concurrency Tests ====================

// TestConcurrentUpdate tests concurrent update operations
func TestConcurrentUpdate(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE concurrent_update (id SEQ, value INT, counter INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert rows to update
	const numRows = 100
	for i := 0; i < numRows; i++ {
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO concurrent_update (value, counter) VALUES (%d, 0)", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numUpdaters = 50
	const updatesPerUpdater = 20

	var wg sync.WaitGroup
	var errors int64

	start := time.Now()

	for i := 0; i < numUpdaters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < updatesPerUpdater; j++ {
				// Update random rows
				_, err := engine.Execute("UPDATE concurrent_update SET counter = counter + 1 WHERE value < 50")
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify counters
	result, err := engine.Execute("SELECT SUM(counter) FROM concurrent_update WHERE value < 50")
	if err != nil {
		t.Fatal(err)
	}
	sumCounter, _ := result.Rows[0].Data[0].ToInt64()
	expectedSum := int64(numUpdaters * updatesPerUpdater * 50)

	t.Logf("Concurrent Update Results:")
	t.Logf("  Updaters: %d", numUpdaters)
	t.Logf("  Updates per updater: %d", updatesPerUpdater)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Sum of counters: %d (expected: %d)", sumCounter, expectedSum)
	t.Logf("  Duration: %v", elapsed)

	if errors > 0 {
		t.Errorf("Encountered %d errors during concurrent updates", errors)
	}
	// Note: Due to lock serialization, counter should be exact
	if sumCounter != expectedSum {
		t.Errorf("Counter mismatch: expected %d, got %d", expectedSum, sumCounter)
	}
}

// TestConcurrentDelete tests concurrent delete operations
func TestConcurrentDelete(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE concurrent_delete (id SEQ, value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert many rows
	const numRows = 1000
	for i := 0; i < numRows; i++ {
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO concurrent_delete (value) VALUES (%d)", i%100))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numDeleters = 10
	var wg sync.WaitGroup
	var deletedCount int64

	start := time.Now()

	// Each goroutine deletes rows with specific value
	for i := 0; i < numDeleters; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			result, err := engine.Execute(fmt.Sprintf("DELETE FROM concurrent_delete WHERE value = %d", val))
			if err != nil {
				t.Errorf("Delete failed: %v", err)
				return
			}
			atomic.AddInt64(&deletedCount, result.RowsAffected)
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Check remaining rows
	result, err := engine.Execute("SELECT COUNT(*) FROM concurrent_delete")
	if err != nil {
		t.Fatal(err)
	}
	remaining, _ := result.Rows[0].Data[0].ToInt64()

	t.Logf("Concurrent Delete Results:")
	t.Logf("  Initial rows: %d", numRows)
	t.Logf("  Deleters: %d", numDeleters)
	t.Logf("  Deleted count: %d", deletedCount)
	t.Logf("  Remaining rows: %d", remaining)
	t.Logf("  Duration: %v", elapsed)

	if int(deletedCount)+int(remaining) != numRows {
		t.Errorf("Row count mismatch: deleted %d + remaining %d != initial %d", deletedCount, remaining, numRows)
	}
}

// ==================== Sequence Auto-Increment Tests ====================

// TestConcurrentAutoIncrement tests concurrent auto-increment sequence
func TestConcurrentAutoIncrement(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE auto_inc_test (id SEQ, data VARCHAR(100))")
	if err != nil {
		t.Fatal(err)
	}

	const numGoroutines = 100
	const insertsPerGoroutine = 20

	var wg sync.WaitGroup
	var mu sync.Mutex
	idMap := make(map[int64]bool)
	var duplicateCount int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < insertsPerGoroutine; j++ {
				result, err := engine.Execute(fmt.Sprintf("INSERT INTO auto_inc_test (data) VALUES ('thread-%d-item-%d')", threadID, j))
				if err != nil {
					t.Errorf("Insert failed: %v", err)
					continue
				}

				mu.Lock()
				if idMap[result.LastInsertID] {
					atomic.AddInt64(&duplicateCount, 1)
					t.Errorf("Duplicate ID detected: %d", result.LastInsertID)
				}
				idMap[result.LastInsertID] = true
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify all IDs are unique and sequential
	result, err := engine.Execute("SELECT COUNT(*) FROM auto_inc_test")
	if err != nil {
		t.Fatal(err)
	}
	count, _ := result.Rows[0].Data[0].ToInt64()
	expected := int64(numGoroutines * insertsPerGoroutine)

	t.Logf("Auto-Increment Concurrency Results:")
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Inserts per goroutine: %d", insertsPerGoroutine)
	t.Logf("  Expected rows: %d", expected)
	t.Logf("  Actual rows: %d", count)
	t.Logf("  Unique IDs: %d", len(idMap))
	t.Logf("  Duplicate IDs: %d", duplicateCount)
	t.Logf("  Duration: %v", elapsed)

	if count != expected {
		t.Errorf("Row count mismatch: expected %d, got %d", expected, count)
	}
	if duplicateCount > 0 {
		t.Errorf("Found %d duplicate auto-increment IDs", duplicateCount)
	}
	if int64(len(idMap)) != expected {
		t.Errorf("ID count mismatch: expected %d unique IDs, got %d", expected, len(idMap))
	}
}

// ==================== Deadlock Detection Tests ====================

// TestDeadlockDetection tests for potential deadlocks
func TestDeadlockDetection(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE deadlock_test (id SEQ, value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert initial data
	for i := 0; i < 100; i++ {
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO deadlock_test (value) VALUES (%d)", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numGoroutines = 50
	const timeout = 30 * time.Second

	var wg sync.WaitGroup
	done := make(chan bool, numGoroutines)
	start := time.Now()

	// Mixed operations that could potentially cause deadlock
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				switch j % 4 {
				case 0:
					engine.Execute(fmt.Sprintf("INSERT INTO deadlock_test (value) VALUES (%d)", id*100+j))
				case 1:
					engine.Execute("SELECT * FROM deadlock_test WHERE value < 50")
				case 2:
					engine.Execute(fmt.Sprintf("UPDATE deadlock_test SET value = value + 1 WHERE id = %d", (j%100)+1))
				case 3:
					engine.Execute(fmt.Sprintf("DELETE FROM deadlock_test WHERE value = %d", id*100+j-3))
				}
			}
			done <- true
		}(i)
	}

	// Wait with timeout
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		elapsed := time.Since(start)
		t.Logf("Deadlock Test Results:")
		t.Logf("  Goroutines: %d", numGoroutines)
		t.Logf("  Completed without deadlock in: %v", elapsed)
	case <-time.After(timeout):
		t.Fatalf("Potential deadlock detected: test did not complete within %v", timeout)
	}
}

// ==================== Data Integrity Tests ====================

// TestConcurrentDataIntegrity verifies data integrity under concurrent access
func TestConcurrentDataIntegrity(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE integrity_test (id INT PRIMARY KEY, value INT, checksum INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert rows with checksums
	const numRows = 100
	for i := 0; i < numRows; i++ {
		checksum := i * 2 // Simple checksum
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO integrity_test (id, value, checksum) VALUES (%d, %d, %d)", i, i, checksum))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numUpdaters = 20
	const updatesPerRound = 10

	var wg sync.WaitGroup
	var errors int64

	for i := 0; i < numUpdaters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < updatesPerRound; j++ {
				// Update value and checksum together
				for k := 0; k < numRows; k++ {
					newValue := j
					newChecksum := newValue * 2
					_, err := engine.Execute(fmt.Sprintf("UPDATE integrity_test SET value = %d, checksum = %d WHERE id = %d", newValue, newChecksum, k))
					if err != nil {
						atomic.AddInt64(&errors, 1)
					}
				}
			}
		}()
	}

	wg.Wait()

	// Verify checksums
	result, err := engine.Execute("SELECT id, value, checksum FROM integrity_test")
	if err != nil {
		t.Fatal(err)
	}

	var integrityErrors int64
	for _, row := range result.Rows {
		id, _ := row.Data[0].ToInt64()
		value, _ := row.Data[1].ToInt64()
		checksum, _ := row.Data[2].ToInt64()
		expectedChecksum := value * 2
		if checksum != expectedChecksum {
			integrityErrors++
			t.Errorf("Integrity error for id %d: checksum=%d, expected=%d", id, checksum, expectedChecksum)
		}
	}

	t.Logf("Data Integrity Test Results:")
	t.Logf("  Rows: %d", numRows)
	t.Logf("  Updaters: %d", numUpdaters)
	t.Logf("  Update errors: %d", errors)
	t.Logf("  Integrity errors: %d", integrityErrors)

	if integrityErrors > 0 {
		t.Errorf("Found %d data integrity errors", integrityErrors)
	}
}

// ==================== Stress Tests ====================

// TestHighConcurrencyStress stress test with many goroutines
func TestHighConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE stress_test (id SEQ, thread_id INT, op_num INT, data VARCHAR(50))")
	if err != nil {
		t.Fatal(err)
	}

	const numGoroutines = 200
	const opsPerThread = 100

	var wg sync.WaitGroup
	var insertCount int64
	var selectCount int64
	var updateCount int64
	var deleteCount int64
	var errors int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < opsPerThread; j++ {
				switch j % 10 {
				case 0, 1, 2, 3, 4: // 50% inserts
					_, err := engine.Execute(fmt.Sprintf("INSERT INTO stress_test (thread_id, op_num, data) VALUES (%d, %d, 'data-%d-%d')", threadID, j, threadID, j))
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&insertCount, 1)
					}
				case 5, 6, 7: // 30% selects
					_, err := engine.Execute(fmt.Sprintf("SELECT * FROM stress_test WHERE thread_id = %d", threadID))
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&selectCount, 1)
					}
				case 8: // 10% updates
					_, err := engine.Execute(fmt.Sprintf("UPDATE stress_test SET data = 'updated-%d-%d' WHERE thread_id = %d AND op_num = %d", threadID, j, threadID, j-1))
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&updateCount, 1)
					}
				case 9: // 10% deletes
					_, err := engine.Execute(fmt.Sprintf("DELETE FROM stress_test WHERE thread_id = %d AND op_num = %d", threadID, j-2))
					if err != nil {
						atomic.AddInt64(&errors, 1)
					} else {
						atomic.AddInt64(&deleteCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Final count
	result, _ := engine.Execute("SELECT COUNT(*) FROM stress_test")
	finalCount, _ := result.Rows[0].Data[0].ToInt64()

	totalOps := insertCount + selectCount + updateCount + deleteCount

	t.Logf("High Concurrency Stress Test Results:")
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Ops per goroutine: %d", opsPerThread)
	t.Logf("  Total operations: %d", totalOps)
	t.Logf("  Inserts: %d", insertCount)
	t.Logf("  Selects: %d", selectCount)
	t.Logf("  Updates: %d", updateCount)
	t.Logf("  Deletes: %d", deleteCount)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Final row count: %d", finalCount)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Throughput: %.2f ops/sec", float64(totalOps)/elapsed.Seconds())

	if errors > int64(numGoroutines*opsPerThread/10) {
		t.Errorf("Too many errors: %d", errors)
	}
}

// TestLongRunningStress runs a longer stress test
func TestLongRunningStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stress test in short mode")
	}

	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE long_stress (id SEQ, value INT, created TIMESTAMP)")
	if err != nil {
		t.Fatal(err)
	}

	const duration = 10 * time.Second
	const numWorkers = 50

	var wg sync.WaitGroup
	var stop int64
	var totalOps int64
	var errors int64

	start := time.Now()

	// Workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for atomic.LoadInt64(&stop) == 0 {
				_, err := engine.Execute(fmt.Sprintf("INSERT INTO long_stress (value) VALUES (%d)", workerID))
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
				atomic.AddInt64(&totalOps, 1)
			}
		}(i)
	}

	// Reader
	wg.Add(1)
	go func() {
		defer wg.Done()
		for atomic.LoadInt64(&stop) == 0 {
			engine.Execute("SELECT COUNT(*) FROM long_stress")
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Run for specified duration
	time.Sleep(duration)
	atomic.StoreInt64(&stop, 1)
	wg.Wait()
	elapsed := time.Since(start)

	result, _ := engine.Execute("SELECT COUNT(*) FROM long_stress")
	finalCount, _ := result.Rows[0].Data[0].ToInt64()

	t.Logf("Long Running Stress Test Results:")
	t.Logf("  Duration: %v", duration)
	t.Logf("  Workers: %d", numWorkers)
	t.Logf("  Total ops: %d", totalOps)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Final row count: %d", finalCount)
	t.Logf("  Actual duration: %v", elapsed)
	t.Logf("  Throughput: %.2f ops/sec", float64(totalOps)/elapsed.Seconds())

	if errors > totalOps/100 {
		t.Errorf("Too many errors: %d out of %d ops", errors, totalOps)
	}
}

// ==================== Race Condition Tests ====================

// TestRaceConditionDetection tests for race conditions
func TestRaceConditionDetection(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE race_test (id SEQ, counter INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert initial row
	_, err = engine.Execute("INSERT INTO race_test (counter) VALUES (0)")
	if err != nil {
		t.Fatal(err)
	}

	const numRacers = 100
	const iterations = 50

	var wg sync.WaitGroup

	// All goroutines try to increment the same counter
	for i := 0; i < numRacers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Read-modify-write pattern (should be protected by lock)
				engine.Execute("UPDATE race_test SET counter = counter + 1 WHERE id = 1")
			}
		}()
	}

	wg.Wait()

	// Verify final counter value
	result, err := engine.Execute("SELECT counter FROM race_test WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	counter, _ := result.Rows[0].Data[0].ToInt64()
	expected := int64(numRacers * iterations)

	t.Logf("Race Condition Test Results:")
	t.Logf("  Racers: %d", numRacers)
	t.Logf("  Iterations per racer: %d", iterations)
	t.Logf("  Expected counter: %d", expected)
	t.Logf("  Actual counter: %d", counter)

	// With proper locking, counter should be exact
	if counter != expected {
		t.Errorf("Race condition detected: expected counter=%d, got counter=%d", expected, counter)
	}
}

// ==================== Memory Pressure Tests ====================

// TestMemoryPressure tests behavior under memory pressure
func TestMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE memory_test (id SEQ, data TEXT)")
	if err != nil {
		t.Fatal(err)
	}

	const numGoroutines = 20
	const insertsPerGoroutine = 100
	const dataSize = 1000 // bytes per row

	var wg sync.WaitGroup
	var errors int64

	// Create large strings
	largeData := make([]byte, dataSize)
	for i := range largeData {
		largeData[i] = 'A' + byte(i%26)
	}
	dataStr := string(largeData)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < insertsPerGoroutine; j++ {
				_, err := engine.Execute(fmt.Sprintf("INSERT INTO memory_test (data) VALUES ('%s')", dataStr))
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	result, _ := engine.Execute("SELECT COUNT(*) FROM memory_test")
	count, _ := result.Rows[0].Data[0].ToInt64()
	expected := int64(numGoroutines * insertsPerGoroutine)

	t.Logf("Memory Pressure Test Results:")
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Inserts per goroutine: %d", insertsPerGoroutine)
	t.Logf("  Data size per row: %d bytes", dataSize)
	t.Logf("  Expected rows: %d", expected)
	t.Logf("  Actual rows: %d", count)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Approximate data: %.2f MB", float64(expected*dataSize)/1024/1024)

	if count != expected {
		t.Errorf("Data loss: expected %d rows, got %d", expected, count)
	}
}

// ==================== Persistence Concurrency Tests ====================

// TestPersistenceConcurrency tests concurrent operations with file-based storage
func TestPersistenceConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping persistence test in short mode")
	}

	dir := t.TempDir()

	config := Config{
		Path:     dir,
		InMemory: false,
	}

	engine, err := OpenWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE TABLE persist_concurrent (id SEQ, value INT, thread_id INT)")
	if err != nil {
		engine.Close()
		t.Fatal(err)
	}

	const numGoroutines = 50
	const opsPerThread = 20

	var wg sync.WaitGroup
	var errors int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			for j := 0; j < opsPerThread; j++ {
				_, err := engine.Execute(fmt.Sprintf("INSERT INTO persist_concurrent (value, thread_id) VALUES (%d, %d)", j, threadID))
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify before close
	result, _ := engine.Execute("SELECT COUNT(*) FROM persist_concurrent")
	countBeforeClose, _ := result.Rows[0].Data[0].ToInt64()

	// Close and reopen
	engine.Close()

	engine2, err := OpenWithConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine2.Close()

	// Verify after reopen
	result, _ = engine2.Execute("SELECT COUNT(*) FROM persist_concurrent")
	countAfterReopen, _ := result.Rows[0].Data[0].ToInt64()

	elapsed := time.Since(start)
	expected := int64(numGoroutines * opsPerThread)

	t.Logf("Persistence Concurrency Test Results:")
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Ops per goroutine: %d", opsPerThread)
	t.Logf("  Expected rows: %d", expected)
	t.Logf("  Rows before close: %d", countBeforeClose)
	t.Logf("  Rows after reopen: %d", countAfterReopen)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)

	if countBeforeClose != expected {
		t.Errorf("Data loss before close: expected %d, got %d", expected, countBeforeClose)
	}
	if countAfterReopen != expected {
		t.Errorf("Data loss after reopen: expected %d, got %d", expected, countAfterReopen)
	}
}

// ==================== Multiple Table Concurrency Tests ====================

// TestMultipleTableConcurrency tests concurrent operations on multiple tables
func TestMultipleTableConcurrency(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create multiple tables
	const numTables = 10
	for i := 0; i < numTables; i++ {
		_, err = engine.Execute(fmt.Sprintf("CREATE TABLE table_%d (id SEQ, value INT)", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numGoroutines = 50
	const opsPerThread = 30

	var wg sync.WaitGroup
	var errors int64

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < opsPerThread; j++ {
				tableID := j % numTables
				_, err := engine.Execute(fmt.Sprintf("INSERT INTO table_%d (value) VALUES (%d)", tableID, goroutineID*100+j))
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Verify each table
	var totalRows int64
	for i := 0; i < numTables; i++ {
		result, _ := engine.Execute(fmt.Sprintf("SELECT COUNT(*) FROM table_%d", i))
		count, _ := result.Rows[0].Data[0].ToInt64()
		totalRows += count
	}

	expected := int64(numGoroutines * opsPerThread)

	t.Logf("Multiple Table Concurrency Test Results:")
	t.Logf("  Tables: %d", numTables)
	t.Logf("  Goroutines: %d", numGoroutines)
	t.Logf("  Ops per goroutine: %d", opsPerThread)
	t.Logf("  Expected total rows: %d", expected)
	t.Logf("  Actual total rows: %d", totalRows)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)

	if totalRows != expected {
		t.Errorf("Data loss: expected %d rows, got %d", expected, totalRows)
	}
}

// ==================== Aggregation Concurrency Tests ====================

// TestAggregateConcurrency tests concurrent aggregate operations
func TestAggregateConcurrency(t *testing.T) {
	engine, err := OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	_, err = engine.Execute("CREATE TABLE agg_concurrent (id SEQ, category VARCHAR(10), value INT)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert test data
	const numRows = 5000
	for i := 0; i < numRows; i++ {
		category := fmt.Sprintf("CAT%d", i%10)
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO agg_concurrent (category, value) VALUES ('%s', %d)", category, i))
		if err != nil {
			t.Fatal(err)
		}
	}

	const numReaders = 30
	const readsPerReader = 20

	var wg sync.WaitGroup
	var errors int64

	queries := []string{
		"SELECT COUNT(*) FROM agg_concurrent",
		"SELECT SUM(value) FROM agg_concurrent",
		"SELECT AVG(value) FROM agg_concurrent",
		"SELECT MIN(value) FROM agg_concurrent",
		"SELECT MAX(value) FROM agg_concurrent",
		"SELECT category, COUNT(*) FROM agg_concurrent GROUP BY category",
	}

	start := time.Now()

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < readsPerReader; j++ {
				query := queries[j%len(queries)]
				_, err := engine.Execute(query)
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Aggregate Concurrency Test Results:")
	t.Logf("  Readers: %d", numReaders)
	t.Logf("  Reads per reader: %d", readsPerReader)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Throughput: %.2f agg queries/sec", float64(numReaders*readsPerReader)/elapsed.Seconds())

	if errors > 0 {
		t.Errorf("Encountered %d errors during aggregate operations", errors)
	}
}

// ==================== Connection Stress Tests ====================

// TestMultipleEngineInstances tests multiple engine instances
func TestMultipleEngineInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multiple engine test in short mode")
	}

	const numEngines = 5
	const opsPerEngine = 100

	var wg sync.WaitGroup
	var errors int64

	start := time.Now()

	for i := 0; i < numEngines; i++ {
		wg.Add(1)
		go func(engineID int) {
			defer wg.Done()

			engine, err := OpenInMemory()
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}
			defer engine.Close()

			_, err = engine.Execute("CREATE TABLE test (id SEQ, value INT)")
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}

			for j := 0; j < opsPerEngine; j++ {
				_, err = engine.Execute(fmt.Sprintf("INSERT INTO test (value) VALUES (%d)", j))
				if err != nil {
					atomic.AddInt64(&errors, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	t.Logf("Multiple Engine Instances Test Results:")
	t.Logf("  Engines: %d", numEngines)
	t.Logf("  Ops per engine: %d", opsPerEngine)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", elapsed)

	if errors > 0 {
		t.Errorf("Encountered %d errors across engines", errors)
	}
}
