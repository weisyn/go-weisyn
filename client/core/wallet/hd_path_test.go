package wallet

import (
	"testing"
)

func TestDefaultDerivationPath(t *testing.T) {
	dp := DefaultDerivationPath()

	if dp.Purpose != BIP44Purpose {
		t.Errorf("Purpose = %d, want %d", dp.Purpose, BIP44Purpose)
	}

	if dp.CoinType != WESCoinType {
		t.Errorf("CoinType = %d, want %d", dp.CoinType, WESCoinType)
	}

	if dp.Account != DefaultAccount {
		t.Errorf("Account = %d, want %d", dp.Account, DefaultAccount)
	}

	if dp.Change != ExternalChain {
		t.Errorf("Change = %d, want %d", dp.Change, ExternalChain)
	}

	if dp.AddressIndex != DefaultAddressIndex {
		t.Errorf("AddressIndex = %d, want %d", dp.AddressIndex, DefaultAddressIndex)
	}
}

func TestDerivationPath_String(t *testing.T) {
	tests := []struct {
		name string
		path *DerivationPath
		want string
	}{
		{
			"default path",
			DefaultDerivationPath(),
			"m/44'/8888'/0'/0/0",
		},
		{
			"account 1",
			NewDerivationPath(1, ExternalChain, 0),
			"m/44'/8888'/1'/0/0",
		},
		{
			"internal chain",
			NewDerivationPath(0, InternalChain, 5),
			"m/44'/8888'/0'/1/5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.path.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseDerivationPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    *DerivationPath
		wantErr bool
	}{
		{
			"full path",
			"m/44'/8888'/0'/0/0",
			DefaultDerivationPath(),
			false,
		},
		{
			"without m prefix",
			"44'/8888'/0'/0/0",
			DefaultDerivationPath(),
			false,
		},
		{
			"uppercase M",
			"M/44'/8888'/0'/0/0",
			DefaultDerivationPath(),
			false,
		},
		{
			"with H for hardened",
			"m/44H/8888H/0H/0/0",
			DefaultDerivationPath(),
			false,
		},
		{
			"account 1",
			"m/44'/8888'/1'/0/0",
			NewDerivationPath(1, ExternalChain, 0),
			false,
		},
		{
			"wrong component count",
			"m/44'/8888'/0'",
			nil,
			true,
		},
		{
			"non-hardened purpose",
			"m/44/8888'/0'/0/0",
			nil,
			true,
		},
		{
			"invalid change value",
			"m/44'/8888'/0'/2/0",
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDerivationPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDerivationPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Purpose != tt.want.Purpose ||
					got.CoinType != tt.want.CoinType ||
					got.Account != tt.want.Account ||
					got.Change != tt.want.Change ||
					got.AddressIndex != tt.want.AddressIndex {
					t.Errorf("ParseDerivationPath() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestDerivationPath_ToUint32Array(t *testing.T) {
	dp := DefaultDerivationPath()
	arr := dp.ToUint32Array()

	if len(arr) != 5 {
		t.Fatalf("ToUint32Array() length = %d, want 5", len(arr))
	}

	// 前三个应该是硬化的
	if arr[0] != BIP44Purpose+HardenedOffset {
		t.Errorf("arr[0] = %d, want %d (hardened purpose)", arr[0], BIP44Purpose+HardenedOffset)
	}

	if arr[1] != WESCoinType+HardenedOffset {
		t.Errorf("arr[1] = %d, want %d (hardened coin type)", arr[1], WESCoinType+HardenedOffset)
	}

	if arr[2] != DefaultAccount+HardenedOffset {
		t.Errorf("arr[2] = %d, want %d (hardened account)", arr[2], DefaultAccount+HardenedOffset)
	}

	// 后两个应该是非硬化的
	if arr[3] != ExternalChain {
		t.Errorf("arr[3] = %d, want %d (change)", arr[3], ExternalChain)
	}

	if arr[4] != DefaultAddressIndex {
		t.Errorf("arr[4] = %d, want %d (address index)", arr[4], DefaultAddressIndex)
	}
}

func TestDerivationPath_With(t *testing.T) {
	dp := DefaultDerivationPath()

	// WithAccount
	newDp := dp.WithAccount(5)
	if newDp.Account != 5 {
		t.Errorf("WithAccount(5).Account = %d, want 5", newDp.Account)
	}
	if dp.Account != DefaultAccount {
		t.Error("WithAccount() modified original path")
	}

	// WithChange
	newDp = dp.WithChange(InternalChain)
	if newDp.Change != InternalChain {
		t.Errorf("WithChange(1).Change = %d, want %d", newDp.Change, InternalChain)
	}

	// WithAddressIndex
	newDp = dp.WithAddressIndex(10)
	if newDp.AddressIndex != 10 {
		t.Errorf("WithAddressIndex(10).AddressIndex = %d, want 10", newDp.AddressIndex)
	}
}

func TestDerivationPath_NextAddress(t *testing.T) {
	dp := DefaultDerivationPath()
	next := dp.NextAddress()

	if next.AddressIndex != 1 {
		t.Errorf("NextAddress().AddressIndex = %d, want 1", next.AddressIndex)
	}

	// 其他字段应该不变
	if next.Account != dp.Account || next.Change != dp.Change {
		t.Error("NextAddress() modified other fields")
	}
}

func TestDerivationPath_IsExternalInternal(t *testing.T) {
	external := NewDerivationPath(0, ExternalChain, 0)
	internal := NewDerivationPath(0, InternalChain, 0)

	if !external.IsExternal() {
		t.Error("IsExternal() = false for external chain")
	}
	if external.IsInternal() {
		t.Error("IsInternal() = true for external chain")
	}

	if internal.IsExternal() {
		t.Error("IsExternal() = true for internal chain")
	}
	if !internal.IsInternal() {
		t.Error("IsInternal() = false for internal chain")
	}
}

func TestDerivationPath_IsWESPath(t *testing.T) {
	wesPath := DefaultDerivationPath()
	if !wesPath.IsWESPath() {
		t.Error("IsWESPath() = false for WES default path")
	}

	// 修改 coin type
	otherPath := &DerivationPath{
		Purpose:      BIP44Purpose,
		CoinType:     0, // Bitcoin
		Account:      0,
		Change:       0,
		AddressIndex: 0,
	}
	if otherPath.IsWESPath() {
		t.Error("IsWESPath() = true for non-WES coin type")
	}
}

func TestDerivationPath_Validate(t *testing.T) {
	tests := []struct {
		name    string
		path    *DerivationPath
		wantErr bool
	}{
		{
			"valid WES path",
			DefaultDerivationPath(),
			false,
		},
		{
			"wrong purpose",
			&DerivationPath{Purpose: 49, CoinType: WESCoinType, Account: 0, Change: 0, AddressIndex: 0},
			true,
		},
		{
			"wrong coin type",
			&DerivationPath{Purpose: BIP44Purpose, CoinType: 0, Account: 0, Change: 0, AddressIndex: 0},
			true,
		},
		{
			"invalid change",
			&DerivationPath{Purpose: BIP44Purpose, CoinType: WESCoinType, Account: 0, Change: 2, AddressIndex: 0},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.path.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHDPathGenerator(t *testing.T) {
	gen := NewHDPathGenerator(0)

	// GenerateReceivePath
	receivePath := gen.GenerateReceivePath(0)
	if receivePath.Change != ExternalChain {
		t.Errorf("GenerateReceivePath() change = %d, want %d", receivePath.Change, ExternalChain)
	}

	// GenerateChangePath
	changePath := gen.GenerateChangePath(0)
	if changePath.Change != InternalChain {
		t.Errorf("GenerateChangePath() change = %d, want %d", changePath.Change, InternalChain)
	}

	// GeneratePaths
	paths := gen.GeneratePaths(ExternalChain, 0, 5)
	if len(paths) != 5 {
		t.Errorf("GeneratePaths() length = %d, want 5", len(paths))
	}
	for i, p := range paths {
		if p.AddressIndex != uint32(i) {
			t.Errorf("GeneratePaths()[%d].AddressIndex = %d, want %d", i, p.AddressIndex, i)
		}
	}
}

func TestWESPathHelpers(t *testing.T) {
	if got := WESDefaultPath(); got != "m/44'/8888'/0'/0/0" {
		t.Errorf("WESDefaultPath() = %s, want m/44'/8888'/0'/0/0", got)
	}

	if got := WESPathForAccount(1); got != "m/44'/8888'/1'/0/0" {
		t.Errorf("WESPathForAccount(1) = %s, want m/44'/8888'/1'/0/0", got)
	}

	if got := WESPathForIndex(0, 5); got != "m/44'/8888'/0'/0/5" {
		t.Errorf("WESPathForIndex(0, 5) = %s, want m/44'/8888'/0'/0/5", got)
	}
}

