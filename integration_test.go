package main

import (
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"txn-service/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicTransactionFlow(t *testing.T) {
	ts := testutil.SetupTestServer(t)
	defer ts.Cleanup()

	account1ID := int64(101)
	account2ID := int64(102)
	initialBalance := "500.00"

	ts.CreateTestAccount(t, account1ID, initialBalance)
	ts.CreateTestAccount(t, account2ID, initialBalance)

	balance1 := ts.GetAccountBalance(t, account1ID)
	balance2 := ts.GetAccountBalance(t, account2ID)

	balance1Float, _ := strconv.ParseFloat(balance1, 64)
	balance2Float, _ := strconv.ParseFloat(balance2, 64)
	expectedBalanceFloat, _ := strconv.ParseFloat(initialBalance, 64)

	assert.Equal(t, expectedBalanceFloat, balance1Float, "Account 1 should have initial balance of 500")
	assert.Equal(t, expectedBalanceFloat, balance2Float, "Account 2 should have initial balance of 500")

	transactionAmount := "100.00"
	transactionID := ts.CreateTransaction(t, account1ID, account2ID, transactionAmount)

	assert.NotEmpty(t, transactionID)

	time.Sleep(1 * time.Second)

	finalBalance1 := ts.GetAccountBalance(t, account1ID)
	finalBalance2 := ts.GetAccountBalance(t, account2ID)

	expectedBalance1Float := 400.0
	expectedBalance2Float := 600.0
	finalBalance1Float, _ := strconv.ParseFloat(finalBalance1, 64)
	finalBalance2Float, _ := strconv.ParseFloat(finalBalance2, 64)

	assert.Equal(t, expectedBalance1Float, finalBalance1Float, "Account 1 balance should be 400 after transaction")
	assert.Equal(t, expectedBalance2Float, finalBalance2Float, "Account 2 balance should be 600 after transaction")
}

func TestInsufficientBalance(t *testing.T) {
	ts := testutil.SetupTestServer(t)
	defer ts.Cleanup()

	account1ID := int64(201)
	account2ID := int64(202)
	initialBalance := "100.00"

	ts.CreateTestAccount(t, account1ID, initialBalance)
	ts.CreateTestAccount(t, account2ID, initialBalance)

	excessiveAmount := "200.00"
	url := fmt.Sprintf("%s/transactions", ts.Server.URL)
	payload := fmt.Sprintf(`{
		"source_account_id": %d,
		"destination_account_id": %d,
		"amount": "%s"
	}`, account1ID, account2ID, excessiveAmount)

	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	require.NoError(t, err)

	assert.NotEqual(t, http.StatusOK, resp.StatusCode, "Transaction with insufficient balance should fail")
	resp.Body.Close()

	balance1 := ts.GetAccountBalance(t, account1ID)
	balance2 := ts.GetAccountBalance(t, account2ID)

	balance1Float, _ := strconv.ParseFloat(balance1, 64)
	balance2Float, _ := strconv.ParseFloat(balance2, 64)
	expectedBalanceFloat, _ := strconv.ParseFloat(initialBalance, 64)

	assert.Equal(t, expectedBalanceFloat, balance1Float, "Account 1 balance should remain unchanged")
	assert.Equal(t, expectedBalanceFloat, balance2Float, "Account 2 balance should remain unchanged")
}

func TestConcurrencyHandling(t *testing.T) {
	ts := testutil.SetupTestServer(t)
	defer ts.Cleanup()

	account1ID := int64(2001)
	account2ID := int64(2002)
	initialBalance := "1000.00"

	ts.CreateTestAccount(t, account1ID, initialBalance)
	ts.CreateTestAccount(t, account2ID, initialBalance)

	balance1 := ts.GetAccountBalance(t, account1ID)
	balance2 := ts.GetAccountBalance(t, account2ID)

	initialBalanceFloat, _ := strconv.ParseFloat(initialBalance, 64)
	balance1Float, _ := strconv.ParseFloat(balance1, 64)
	balance2Float, _ := strconv.ParseFloat(balance2, 64)

	assert.Equal(t, initialBalanceFloat, balance1Float, "Account 1 should have initial balance of 1000")
	assert.Equal(t, initialBalanceFloat, balance2Float, "Account 2 should have initial balance of 1000")

	numTransactions := 300
	transactionAmount := "1.00"

	var wg sync.WaitGroup
	wg.Add(numTransactions)

	errorChan := make(chan error, numTransactions)

	for i := 0; i < numTransactions; i++ {
		go func(transactionNum int) {
			defer wg.Done()

			var sourceID, destID int64
			if transactionNum%2 == 0 {
				sourceID = account1ID
				destID = account2ID
			} else {
				sourceID = account2ID
				destID = account1ID
			}

			transactionID := ts.CreateTransaction(t, sourceID, destID, transactionAmount)

			if transactionID == "" {
				errorChan <- fmt.Errorf("transaction %d failed: empty transaction ID", transactionNum)
				return
			}

			if transactionNum%100 == 0 {
				fmt.Printf("Progress: %d/%d transactions\n", transactionNum, numTransactions)
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	errorCount := 0
	for err := range errorChan {
		errorCount++
		t.Errorf("Transaction error: %v", err)
	}

	if errorCount > 0 {
		fmt.Printf("Found %d transaction errors (this is expected under high concurrency)\n", errorCount)
	}

	fmt.Printf("All transactions completed. Verifying final balances...\n")

	time.Sleep(2 * time.Second)

	finalBalance1 := ts.GetAccountBalance(t, account1ID)
	finalBalance2 := ts.GetAccountBalance(t, account2ID)

	fmt.Printf("Final balance - Account 1: %s, Account 2: %s\n", finalBalance1, finalBalance2)

	expectedTotal, _ := new(big.Float).SetString("2000.00")
	balance1Float, _ = strconv.ParseFloat(finalBalance1, 64)
	balance2Float, _ = strconv.ParseFloat(finalBalance2, 64)

	actualTotal := balance1Float + balance2Float

	assert.Equal(t, 0, expectedTotal.Cmp(big.NewFloat(actualTotal)),
		"Total money in system should remain constant. Expected: %s, Actual: %s",
		expectedTotal.Text('f', 2), big.NewFloat(actualTotal).Text('f', 2))

	balance1Num, _ := strconv.ParseFloat(finalBalance1, 64)
	balance2Num, _ := strconv.ParseFloat(finalBalance2, 64)

	assert.GreaterOrEqual(t, balance1Num, 0.0, "Account 1 balance should not be negative")
	assert.GreaterOrEqual(t, balance2Num, 0.0, "Account 2 balance should not be negative")
	assert.LessOrEqual(t, balance1Num, 2000.0, "Account 1 balance should not exceed 2000")
	assert.LessOrEqual(t, balance2Num, 2000.0, "Account 2 balance should not exceed 2000")

	successfulTransactions := 300 - errorCount
	fmt.Printf("Concurrency test results:\n")
	fmt.Printf("- Successful transactions: %d\n", successfulTransactions)
	fmt.Printf("- Failed transactions: %d\n", errorCount)
	fmt.Printf("- Final balance - Account 1: %s\n", finalBalance1)
	fmt.Printf("- Final balance - Account 2: %s\n", finalBalance2)
	fmt.Printf("- Total money in system: %s\n", big.NewFloat(actualTotal).Text('f', 2))

	fmt.Printf("Concurrency test completed successfully!\n")
}

func TestHighConcurrencySuccessRate(t *testing.T) {
	ts := testutil.SetupTestServer(t)
	defer ts.Cleanup()

	account1ID := int64(6001)
	account2ID := int64(6002)
	initialBalance := "10000.00"

	ts.CreateTestAccount(t, account1ID, initialBalance)
	ts.CreateTestAccount(t, account2ID, initialBalance)

	balance1 := ts.GetAccountBalance(t, account1ID)
	balance2 := ts.GetAccountBalance(t, account2ID)

	balance1Float, _ := strconv.ParseFloat(balance1, 64)
	balance2Float, _ := strconv.ParseFloat(balance2, 64)
	expectedBalanceFloat, _ := strconv.ParseFloat(initialBalance, 64)

	assert.Equal(t, expectedBalanceFloat, balance1Float)
	assert.Equal(t, expectedBalanceFloat, balance2Float)

	fmt.Printf("Starting high concurrency success rate test with initial balances - Account 1: %s, Account 2: %s\n", balance1, balance2)

	numTransactions := 500
	transactionAmount := "1.00"
	totalTransactions := numTransactions

	var wg sync.WaitGroup
	errorChan := make(chan error, totalTransactions)
	successChan := make(chan bool, totalTransactions)

	createTransaction := func(transactionNum int) {
		defer wg.Done()

		transactionID := ts.CreateTransaction(t, account1ID, account2ID, transactionAmount)

		if transactionID == "" {
			errorChan <- fmt.Errorf("transaction %d failed: empty transaction ID", transactionNum)
		} else {
			successChan <- true
		}
	}

	for i := 0; i < numTransactions; i++ {
		wg.Add(1)
		go createTransaction(i)
	}

	wg.Wait()
	close(errorChan)
	close(successChan)

	successCount := 0
	for range successChan {
		successCount++
	}

	errorCount := 0
	for err := range errorChan {
		errorCount++
		fmt.Printf("Transaction error: %v\n", err)
	}

	successRate := float64(successCount) / float64(totalTransactions) * 100

	fmt.Printf("High concurrency success rate test results:\n")
	fmt.Printf("- Total transactions attempted: %d\n", totalTransactions)
	fmt.Printf("- Successful transactions: %d\n", successCount)
	fmt.Printf("- Failed transactions: %d\n", errorCount)
	fmt.Printf("- Success rate: %.2f%%\n", successRate)

	time.Sleep(2 * time.Second)

	finalBalance1 := ts.GetAccountBalance(t, account1ID)
	finalBalance2 := ts.GetAccountBalance(t, account2ID)

	finalBalance1Float, _ := strconv.ParseFloat(finalBalance1, 64)
	finalBalance2Float, _ := strconv.ParseFloat(finalBalance2, 64)

	expectedBalance1 := 10000.0 - float64(successCount)
	expectedBalance2 := 10000.0 + float64(successCount)

	fmt.Printf("Final balance - Account 1: %s (expected: %.2f)\n", finalBalance1, expectedBalance1)
	fmt.Printf("Final balance - Account 2: %s (expected: %.2f)\n", finalBalance2, expectedBalance2)

	assert.Equal(t, expectedBalance1, finalBalance1Float,
		"Account 1 balance should be 10000 - %d = %.2f", successCount, expectedBalance1)

	assert.Equal(t, expectedBalance2, finalBalance2Float,
		"Account 2 balance should be 10000 + %d = %.2f", successCount, expectedBalance2)

	expectedTotal := 20000.0
	actualTotal := finalBalance1Float + finalBalance2Float
	assert.Equal(t, expectedTotal, actualTotal, "Total money in system should remain constant")
	fmt.Printf("Total money in system: %.2f (should be 20000.00)\n", actualTotal)
	fmt.Printf("High concurrency success rate test completed successfully!\n")
}

func TestMultiAccountTransactionFlow(t *testing.T) {
	ts := testutil.SetupTestServer(t)
	defer ts.Cleanup()

	account1ID := int64(9001)
	account2ID := int64(9002)
	account3ID := int64(9003)
	initialBalance := "1000.00"

	ts.CreateTestAccount(t, account1ID, initialBalance)
	ts.CreateTestAccount(t, account2ID, initialBalance)
	ts.CreateTestAccount(t, account3ID, initialBalance)

	balance1 := ts.GetAccountBalance(t, account1ID)
	balance2 := ts.GetAccountBalance(t, account2ID)
	balance3 := ts.GetAccountBalance(t, account3ID)

	balance1Float, _ := strconv.ParseFloat(balance1, 64)
	balance2Float, _ := strconv.ParseFloat(balance2, 64)
	balance3Float, _ := strconv.ParseFloat(balance3, 64)
	expectedBalanceFloat, _ := strconv.ParseFloat(initialBalance, 64)

	assert.Equal(t, expectedBalanceFloat, balance1Float)
	assert.Equal(t, expectedBalanceFloat, balance2Float)
	assert.Equal(t, expectedBalanceFloat, balance3Float)

	fmt.Printf("Starting multi-account transaction flow test\n")
	fmt.Printf("Initial balances - Account 1: %s, Account 2: %s, Account 3: %s\n", balance1, balance2, balance3)

	numTransactions := 300
	transactionAmount := "10.00"
	transactionsPerDirection := numTransactions / 3

	var wg sync.WaitGroup
	errorChan := make(chan error, numTransactions)
	successChan := make(chan bool, numTransactions)

	createRotationTransactions := func() {
		defer wg.Done()

		for i := 0; i < transactionsPerDirection; i++ {
			transactionID := ts.CreateTransaction(t, account1ID, account2ID, transactionAmount)
			if transactionID == "" {
				errorChan <- fmt.Errorf("rotation 1->2, transaction %d failed: empty transaction ID", i)
			} else {
				successChan <- true
			}

			transactionID = ts.CreateTransaction(t, account2ID, account3ID, transactionAmount)
			if transactionID == "" {
				errorChan <- fmt.Errorf("rotation 2->3, transaction %d failed: empty transaction ID", i)
			} else {
				successChan <- true
			}

			transactionID = ts.CreateTransaction(t, account3ID, account1ID, transactionAmount)
			if transactionID == "" {
				errorChan <- fmt.Errorf("rotation 3->1, transaction %d failed: empty transaction ID", i)
			} else {
				successChan <- true
			}
		}
	}

	wg.Add(1)
	go createRotationTransactions()

	wg.Wait()
	close(errorChan)
	close(successChan)

	successCount := 0
	for range successChan {
		successCount++
	}

	errorCount := 0
	for err := range errorChan {
		errorCount++
		if errorCount <= 5 {
			fmt.Printf("Transaction error: %v\n", err)
		} else if errorCount == 6 {
			fmt.Printf("... (showing only first 5 errors)\n")
		}
	}

	successRate := float64(successCount) / float64(numTransactions) * 100

	fmt.Printf("Multi-account transaction flow test results:\n")
	fmt.Printf("- Total transactions attempted: %d\n", numTransactions)
	fmt.Printf("- Successful transactions: %d\n", successCount)
	fmt.Printf("- Failed transactions: %d\n", errorCount)
	fmt.Printf("- Success rate: %.2f%%\n", successRate)

	time.Sleep(2 * time.Second)

	finalBalance1 := ts.GetAccountBalance(t, account1ID)
	finalBalance2 := ts.GetAccountBalance(t, account2ID)
	finalBalance3 := ts.GetAccountBalance(t, account3ID)

	finalBalance1Float, _ := strconv.ParseFloat(finalBalance1, 64)
	finalBalance2Float, _ := strconv.ParseFloat(finalBalance2, 64)
	finalBalance3Float, _ := strconv.ParseFloat(finalBalance3, 64)

	expectedFinalBalance := 1000.0

	fmt.Printf("Final balance - Account 1: %s (expected: %.2f)\n", finalBalance1, expectedFinalBalance)
	fmt.Printf("Final balance - Account 2: %s (expected: %.2f)\n", finalBalance2, expectedFinalBalance)
	fmt.Printf("Final balance - Account 3: %s (expected: %.2f)\n", finalBalance3, expectedFinalBalance)

	assert.Equal(t, expectedFinalBalance, finalBalance1Float, "Account 1 should return to initial balance")
	assert.Equal(t, expectedFinalBalance, finalBalance2Float, "Account 2 should return to initial balance")
	assert.Equal(t, expectedFinalBalance, finalBalance3Float, "Account 3 should return to initial balance")

	expectedTotal := 3000.0
	actualTotal := finalBalance1Float + finalBalance2Float + finalBalance3Float
	assert.Equal(t, expectedTotal, actualTotal, "Total money in system should remain constant")

	fmt.Printf("Total money in system: %.2f (should be 3000.00)\n", actualTotal)
	fmt.Printf("Multi-account transaction flow test completed successfully!\n")
}
