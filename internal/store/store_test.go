package store

import (
	"os"
	"testing"
)

func setupTestStore(t *testing.T) (*Store, func()) {
	t.Helper()

	// Create temp file for test database
	f, err := os.CreateTemp("", "trade-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	dbPath := f.Name()
	f.Close()

	store, err := New(dbPath)
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("failed to create store: %v", err)
	}

	cleanup := func() {
		store.Close()
		os.Remove(dbPath)
	}

	return store, cleanup
}

// ==================== USER TESTS ====================

func TestCreateUser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, err := store.CreateUser("alice", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.ID == "" {
		t.Error("expected user ID to be set")
	}
	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}
	if user.PasswordHash == "" {
		t.Error("expected password hash to be set")
	}
	if user.PasswordHash == "password123" {
		t.Error("password should be hashed, not stored in plain text")
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.CreateUser("alice", "password123")
	if err != nil {
		t.Fatalf("first CreateUser failed: %v", err)
	}

	_, err = store.CreateUser("alice", "different")
	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got %v", err)
	}
}

func TestAuthenticateUser(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	_, err := store.CreateUser("alice", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Successful auth
	user, err := store.AuthenticateUser("alice", "password123")
	if err != nil {
		t.Fatalf("AuthenticateUser failed: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}

	// Wrong password
	_, err = store.AuthenticateUser("alice", "wrongpassword")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// User not found
	_, err = store.AuthenticateUser("bob", "password123")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetUserByID(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	created, err := store.CreateUser("alice", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user, err := store.GetUserByID(created.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected username 'alice', got '%s'", user.Username)
	}

	// Not found
	_, err = store.GetUserByID("nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetAccountByUserID(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, err := store.CreateUser("alice", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	acc, err := store.GetAccountByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetAccountByUserID failed: %v", err)
	}
	if acc.UserID != user.ID {
		t.Errorf("expected UserID '%s', got '%s'", user.ID, acc.UserID)
	}
	if acc.Balance != StartingBalance {
		t.Errorf("expected starting balance %d, got %d", StartingBalance, acc.Balance)
	}

	// Not found
	_, err = store.GetAccountByUserID("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestGetAccountByID(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, err := store.CreateUser("alice", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	accByUser, err := store.GetAccountByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetAccountByUserID failed: %v", err)
	}

	acc, err := store.GetAccountByID(accByUser.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}
	if acc.ID != accByUser.ID {
		t.Errorf("expected ID '%s', got '%s'", accByUser.ID, acc.ID)
	}

	// Not found
	_, err = store.GetAccountByID("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

// ==================== POSITION TESTS ====================

func TestGetPosition(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Non-existing position returns empty position
	pos, err := store.GetPosition(acc.ID, "FAKE")
	if err != nil {
		t.Fatalf("GetPosition failed: %v", err)
	}
	if pos.Quantity != 0 {
		t.Errorf("expected quantity 0, got %d", pos.Quantity)
	}
	if pos.AccountID != acc.ID {
		t.Errorf("expected account ID '%s', got '%s'", acc.ID, pos.AccountID)
	}
}

func TestCheckMarginForOrder(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy within margin (100 shares @ $100 = $10,000 < $1M)
	err := store.CheckMarginForOrder(acc.ID, "FAKE", "buy", 100, 10000)
	if err != nil {
		t.Errorf("expected no error for order within margin, got %v", err)
	}

	// Buy exceeding margin (100,000 shares @ $100 = $10M > $1M)
	err = store.CheckMarginForOrder(acc.ID, "FAKE", "buy", 100000, 10000)
	if err != ErrInsufficientMargin {
		t.Errorf("expected ErrInsufficientMargin, got %v", err)
	}
}

func TestCheckMarginWithExistingPosition(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Open a long position first
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 5000) // 5000 shares @ $100

	// Adding more should still work within margin
	err := store.CheckMarginForOrder(acc.ID, "FAKE", "buy", 1000, 10000)
	if err != nil {
		t.Errorf("expected no error for additional order within margin, got %v", err)
	}

	// Selling reduces position, should be allowed
	err = store.CheckMarginForOrder(acc.ID, "FAKE", "sell", 3000, 10000)
	if err != nil {
		t.Errorf("expected no error for reducing position, got %v", err)
	}
}

func TestUpdatePositionOnTrade_OpenLong(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy 100 shares @ $100
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}
	if pnl != 0 {
		t.Errorf("expected 0 realized P&L for opening position, got %d", pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 100 {
		t.Errorf("expected quantity 100, got %d", pos.Quantity)
	}
	if pos.AvgPrice != 10000 {
		t.Errorf("expected avg price 10000, got %d", pos.AvgPrice)
	}
}

func TestUpdatePositionOnTrade_AddToLong(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy 100 @ $100
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	// Buy 100 @ $110
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 11000, 100)

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 200 {
		t.Errorf("expected quantity 200, got %d", pos.Quantity)
	}
	// Avg price should be (100*$100 + 100*$110) / 200 = $105
	expectedAvg := int64(10500)
	if pos.AvgPrice != expectedAvg {
		t.Errorf("expected avg price %d, got %d", expectedAvg, pos.AvgPrice)
	}
}

func TestUpdatePositionOnTrade_CloseLongWithProfit(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy 100 @ $100
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	// Sell 100 @ $120 (profit of $20 per share)
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 12000, 100)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}

	expectedPnL := int64(100 * 2000) // 100 shares * $20 profit
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 0 {
		t.Errorf("expected flat position, got %d", pos.Quantity)
	}

	// Check balance increased
	accAfter, _ := store.GetAccountByID(acc.ID)
	expectedBalance := StartingBalance + expectedPnL
	if accAfter.Balance != expectedBalance {
		t.Errorf("expected balance %d, got %d", expectedBalance, accAfter.Balance)
	}
}

func TestUpdatePositionOnTrade_CloseLongWithLoss(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy 100 @ $100
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	// Sell 100 @ $80 (loss of $20 per share)
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 8000, 100)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}

	expectedPnL := int64(100 * -2000) // 100 shares * $20 loss
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}
}

func TestUpdatePositionOnTrade_OpenShort(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Sell 100 shares short @ $100
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 10000, 100)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}
	if pnl != 0 {
		t.Errorf("expected 0 P&L for opening short, got %d", pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != -100 {
		t.Errorf("expected quantity -100, got %d", pos.Quantity)
	}
	if pos.AvgPrice != 10000 {
		t.Errorf("expected avg price 10000, got %d", pos.AvgPrice)
	}
}

func TestUpdatePositionOnTrade_CloseShortWithProfit(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Sell 100 short @ $100
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 10000, 100)
	// Buy to cover @ $80 (profit on short)
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 8000, 100)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}

	expectedPnL := int64(100 * 2000) // Sold at $100, covered at $80 = $20 profit per share
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 0 {
		t.Errorf("expected flat position, got %d", pos.Quantity)
	}
}

func TestUpdatePositionOnTrade_CloseShortWithLoss(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Sell 100 short @ $100
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 10000, 100)
	// Buy to cover @ $120 (loss on short)
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 12000, 100)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}

	expectedPnL := int64(100 * -2000) // Sold at $100, covered at $120 = $20 loss per share
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}
}

func TestUpdatePositionOnTrade_LongToShortReversal(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy 100 @ $100 (go long)
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)

	// Sell 150 @ $120 - closes long and opens short
	// Should realize profit on closing 100 shares, then be short 50
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 12000, 150)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}

	// Realized P&L should only be on the 100 shares that closed the long
	// 100 shares * ($120 - $100) = $2000 per share * 100 = $200,000
	expectedPnL := int64(100 * 2000)
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != -50 {
		t.Errorf("expected quantity -50 (short), got %d", pos.Quantity)
	}
	// Avg price for new short should be at the sell price
	if pos.AvgPrice != 12000 {
		t.Errorf("expected avg price 12000 for new short, got %d", pos.AvgPrice)
	}
}

func TestUpdatePositionOnTrade_ShortToLongReversal(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Sell 100 @ $100 (go short)
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 10000, 100)

	// Buy 150 @ $80 - covers short and opens long
	// Should realize profit on covering 100 shares, then be long 50
	pnl, err := store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 8000, 150)
	if err != nil {
		t.Fatalf("UpdatePositionOnTrade failed: %v", err)
	}

	// Realized P&L should only be on the 100 shares that covered the short
	// 100 shares * ($100 - $80) = $2000 per share * 100 = $200,000
	expectedPnL := int64(100 * 2000)
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 50 {
		t.Errorf("expected quantity 50 (long), got %d", pos.Quantity)
	}
	// Avg price for new long should be at the buy price
	if pos.AvgPrice != 8000 {
		t.Errorf("expected avg price 8000 for new long, got %d", pos.AvgPrice)
	}
}

func TestUpdatePositionOnTrade_PartialClose(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Buy 100 @ $100
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	// Sell 50 @ $120
	pnl, _ := store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 12000, 50)

	expectedPnL := int64(50 * 2000) // 50 shares * $20 profit
	if pnl != expectedPnL {
		t.Errorf("expected P&L %d, got %d", expectedPnL, pnl)
	}

	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 50 {
		t.Errorf("expected quantity 50, got %d", pos.Quantity)
	}
	// Avg price should stay at original $100
	if pos.AvgPrice != 10000 {
		t.Errorf("expected avg price 10000, got %d", pos.AvgPrice)
	}
}

func TestGetAllPositions(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Create some positions
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	store.UpdatePositionOnTrade(acc.ID, "AAPL", "sell", 15000, 50)

	positions, err := store.GetAllPositions(acc.ID)
	if err != nil {
		t.Fatalf("GetAllPositions failed: %v", err)
	}
	if len(positions) != 2 {
		t.Errorf("expected 2 positions, got %d", len(positions))
	}
}

func TestGetTradeHistory(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Make some trades
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 11000, 50)
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 12000, 50)

	trades, err := store.GetTradeHistory(acc.ID, 10)
	if err != nil {
		t.Fatalf("GetTradeHistory failed: %v", err)
	}
	if len(trades) != 3 {
		t.Errorf("expected 3 trades, got %d", len(trades))
	}

	// Should be in reverse chronological order
	if trades[0].Price != 12000 {
		t.Errorf("expected most recent trade first")
	}
}

// ==================== SETTLEMENT TESTS ====================

func TestSettleAccount_Profit(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Create a profitable position: bought at $100, mark at $120
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)

	result, err := store.SettleAccount(acc.ID, "FAKE", 12000)
	if err != nil {
		t.Fatalf("SettleAccount failed: %v", err)
	}

	if result.IsBankrupt {
		t.Error("expected not bankrupt")
	}
	if result.WasReset {
		t.Error("expected no reset for profitable account")
	}

	// Position should be cleared
	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 0 {
		t.Errorf("expected position cleared after settlement, got %d", pos.Quantity)
	}

	// Balance should include unrealized P&L
	accAfter, _ := store.GetAccountByID(acc.ID)
	expectedBalance := StartingBalance + (100 * 2000) // 100 shares * $20 gain
	if accAfter.Balance != expectedBalance {
		t.Errorf("expected balance %d, got %d", expectedBalance, accAfter.Balance)
	}
}

func TestSettleAccount_Loss(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Create a losing position: bought at $100, mark at $80
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)

	result, err := store.SettleAccount(acc.ID, "FAKE", 8000)
	if err != nil {
		t.Fatalf("SettleAccount failed: %v", err)
	}

	if result.IsBankrupt {
		t.Error("expected not bankrupt")
	}

	// Balance should reflect the loss
	accAfter, _ := store.GetAccountByID(acc.ID)
	expectedBalance := StartingBalance + (100 * -2000) // 100 shares * $20 loss
	if accAfter.Balance != expectedBalance {
		t.Errorf("expected balance %d, got %d", expectedBalance, accAfter.Balance)
	}
}

func TestSettleAccount_Bankruptcy(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Create a massive losing position to trigger bankruptcy
	// Buy 10000 shares at $100 with $1M margin
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 10000)

	// Mark at $1 (99% loss) - this should bankrupt the account
	// Loss = 10000 * ($100 - $1) = 10000 * $99 = $990,000 loss
	// Remaining = $1,000,000 - $990,000 = $10,000... still positive
	// Let's use a position that will actually bankrupt
	// Need loss > $1M. With mark at 0, loss = 10000 * $100 = $1M exactly
	// Mark at negative won't work. Let's short a huge position instead.

	// Reset and try again with a different approach
	store.ResetAccount(acc.ID)

	// Short 20000 shares at $50
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 5000, 20000)

	// Mark at $100 - loss on short = 20000 * ($100 - $50) = $1M loss
	// Balance = $1M, loss = $1M, final = 0 (bankrupt)
	result, err := store.SettleAccount(acc.ID, "FAKE", 10000)
	if err != nil {
		t.Fatalf("SettleAccount failed: %v", err)
	}

	if !result.IsBankrupt {
		t.Error("expected bankruptcy")
	}
	if !result.WasReset {
		t.Error("expected account reset after bankruptcy")
	}

	// Balance should be reset to starting balance
	accAfter, _ := store.GetAccountByID(acc.ID)
	if accAfter.Balance != StartingBalance {
		t.Errorf("expected balance reset to %d, got %d", StartingBalance, accAfter.Balance)
	}

	// Position should be cleared
	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 0 {
		t.Errorf("expected position cleared after bankruptcy, got %d", pos.Quantity)
	}
}

func TestSettleAllAccounts(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create multiple users
	user1, _ := store.CreateUser("alice", "pass")
	user2, _ := store.CreateUser("bob", "pass")
	acc1, _ := store.GetAccountByUserID(user1.ID)
	acc2, _ := store.GetAccountByUserID(user2.ID)

	// Give them positions
	store.UpdatePositionOnTrade(acc1.ID, "FAKE", "buy", 10000, 100)
	store.UpdatePositionOnTrade(acc2.ID, "FAKE", "sell", 10000, 100)

	results, err := store.SettleAllAccounts("FAKE", 12000)
	if err != nil {
		t.Fatalf("SettleAllAccounts failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestGetLeaderboard(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Create users with different P&Ls
	user1, _ := store.CreateUser("alice", "pass")
	user2, _ := store.CreateUser("bob", "pass")
	user3, _ := store.CreateUser("charlie", "pass")

	acc1, _ := store.GetAccountByUserID(user1.ID)
	acc2, _ := store.GetAccountByUserID(user2.ID)
	acc3, _ := store.GetAccountByUserID(user3.ID)

	// Alice makes profit
	store.UpdatePositionOnTrade(acc1.ID, "FAKE", "buy", 10000, 100)
	store.UpdatePositionOnTrade(acc1.ID, "FAKE", "sell", 15000, 100) // +$5000 profit

	// Bob makes bigger profit
	store.UpdatePositionOnTrade(acc2.ID, "FAKE", "buy", 10000, 200)
	store.UpdatePositionOnTrade(acc2.ID, "FAKE", "sell", 20000, 200) // +$20000 profit

	// Charlie breaks even (no trades)
	_ = acc3

	leaderboard, err := store.GetLeaderboard(10)
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	if len(leaderboard) != 3 {
		t.Errorf("expected 3 entries, got %d", len(leaderboard))
	}

	// Bob should be first (highest P&L)
	if leaderboard[0].Username != "bob" {
		t.Errorf("expected bob first, got %s", leaderboard[0].Username)
	}
	// Alice should be second
	if leaderboard[1].Username != "alice" {
		t.Errorf("expected alice second, got %s", leaderboard[1].Username)
	}
}

func TestResetAccount(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// Trade to modify balance
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "buy", 10000, 100)
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 12000, 100)

	// Reset
	err := store.ResetAccount(acc.ID)
	if err != nil {
		t.Fatalf("ResetAccount failed: %v", err)
	}

	// Check balance reset
	accAfter, _ := store.GetAccountByID(acc.ID)
	if accAfter.Balance != StartingBalance {
		t.Errorf("expected balance %d, got %d", StartingBalance, accAfter.Balance)
	}

	// Check positions cleared
	pos, _ := store.GetPosition(acc.ID, "FAKE")
	if pos.Quantity != 0 {
		t.Errorf("expected position cleared, got %d", pos.Quantity)
	}
}

func TestCheckBankruptcy(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	user, _ := store.CreateUser("alice", "pass")
	acc, _ := store.GetAccountByUserID(user.ID)

	// No position - not bankrupt
	bankrupt, balance, err := store.CheckBankruptcy(acc.ID, "FAKE", 10000)
	if err != nil {
		t.Fatalf("CheckBankruptcy failed: %v", err)
	}
	if bankrupt {
		t.Error("expected not bankrupt with no position")
	}
	if balance != StartingBalance {
		t.Errorf("expected balance %d, got %d", StartingBalance, balance)
	}

	// Create underwater short position
	store.UpdatePositionOnTrade(acc.ID, "FAKE", "sell", 5000, 20000) // Short 20k @ $50

	// Mark at $100 - massive loss
	bankrupt, balance, err = store.CheckBankruptcy(acc.ID, "FAKE", 10000)
	if err != nil {
		t.Fatalf("CheckBankruptcy failed: %v", err)
	}
	if !bankrupt {
		t.Error("expected bankrupt with underwater short")
	}
	if balance > 0 {
		t.Errorf("expected negative or zero balance, got %d", balance)
	}
}

// ==================== MIGRATION TESTS ====================

func TestMigrationStatus(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// After New(), all migrations should be applied
	applied, pending, err := store.MigrationStatus()
	if err != nil {
		t.Fatalf("MigrationStatus failed: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected no pending migrations, got %d", len(pending))
	}

	// Should have at least the initial migrations applied
	if len(applied) < 2 {
		t.Errorf("expected at least 2 applied migrations, got %d", len(applied))
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	// Running Migrate() again should be a no-op
	err := store.Migrate()
	if err != nil {
		t.Fatalf("second Migrate() failed: %v", err)
	}

	_, pending, err := store.MigrationStatus()
	if err != nil {
		t.Fatalf("MigrationStatus failed: %v", err)
	}

	if len(pending) != 0 {
		t.Errorf("expected no pending migrations after re-run, got %d", len(pending))
	}

	// Verify data is still intact
	_, err = store.CreateUser("test", "pass")
	if err != nil {
		t.Fatalf("CreateUser failed after migration re-run: %v", err)
	}
}

func TestMigrationVersionsAreSequential(t *testing.T) {
	// Verify migrations are in order
	for i, m := range migrations {
		expectedVersion := i + 1
		if m.Version != expectedVersion {
			t.Errorf("migration %d has version %d, expected %d", i, m.Version, expectedVersion)
		}
	}
}
