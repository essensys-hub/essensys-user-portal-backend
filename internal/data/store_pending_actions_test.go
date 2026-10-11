package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/domain"
	"github.com/jmoiron/sqlx"
)

type pendingRow struct {
	guid   string
	params []domain.ExchangeKV
}

func lightCmd(k, mask int) []domain.ExchangeKV {
	return domain.ExpandLegacyScenarioBlock([]domain.ExchangeKV{{K: k, V: strconv.Itoa(mask)}})
}

// mockPending prepares FetchPendingActionsForMachine: the SELECT returns rows
// in creation order, then one UPDATE ... delivered per row.
func mockPending(t *testing.T, machineID int, rows []pendingRow) (*PortalStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	result := sqlmock.NewRows([]string{"guid", "user_id", "machine_id", "params", "status", "created_at"})
	start := time.Date(2026, 10, 7, 16, 0, 41, 0, time.UTC)
	for i, r := range rows {
		raw, _ := json.Marshal(r.params)
		result.AddRow(r.guid, 1, machineID, raw, domain.ActionStatusPending, start.Add(time.Duration(i)*time.Millisecond))
	}
	mock.ExpectQuery(`SELECT guid, user_id, machine_id, params, status, created_at\s+FROM cloud_actions`).
		WithArgs(domain.ActionStatusPending, 20, machineID).
		WillReturnRows(result)
	for _, r := range rows {
		mock.ExpectExec(`UPDATE cloud_actions SET status = \$1, delivered_at = NOW\(\) WHERE guid = \$2`).
			WithArgs(domain.ActionStatusDelivered, r.guid).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	return NewPortalStore(sqlx.NewDb(db, "sqlmock")), mock
}

func valuesOf(params []domain.ExchangeKV) map[int]string {
	m := make(map[int]string, len(params))
	for _, p := range params {
		m[p.K] = p.V
	}
	return m
}

func TestFetchPending_bloc_et_scenario_livres_separement_sous_30_parametres(t *testing.T) {
	scenarioWrite := []domain.ExchangeKV{}
	for k := 591; k < 591+30; k++ {
		scenarioWrite = append(scenarioWrite, domain.ExchangeKV{K: k, V: "0"})
	}
	launch := []domain.ExchangeKV{{K: domain.IndexScenario, V: "3"}}
	store, mock := mockPending(t, 31, []pendingRow{
		{"a1", lightCmd(616, 4)},
		{"a2", lightCmd(616, 1)},
		{"s1", scenarioWrite},
		{"a3", lightCmd(613, 64)},
		{"l1", launch},
	})

	got, err := store.FetchPendingActionsForMachine(context.Background(), 31, 20)
	if err != nil {
		t.Fatal(err)
	}
	var guids []string
	for _, a := range got {
		guids = append(guids, a.GUID)
		if len(a.Params) > domain.MaxFirmwareParamsPerAction {
			t.Fatalf("action %s has %d params (> %d)", a.GUID, len(a.Params), domain.MaxFirmwareParamsPerAction)
		}
	}
	if want := "[a2 s1 a3 l1]"; fmt.Sprint(guids) != want {
		t.Fatalf("delivery order = %v, want %s", guids, want)
	}
	if v := valuesOf(got[0].Params); v[616] != "5" || v[domain.IndexScenario] != "1" {
		t.Fatalf("first block should carry 616=5 and 590=1, got 616=%q 590=%q", v[616], v[domain.IndexScenario])
	}
	if v := valuesOf(got[3].Params); v[domain.IndexScenario] != "3" {
		t.Fatalf("scenario launch must be delivered unchanged, got %q", v[domain.IndexScenario])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFetchPending_une_seule_action_livree_telle_quelle(t *testing.T) {
	cmd := lightCmd(613, 64)
	store, mock := mockPending(t, 31, []pendingRow{{"only", cmd}})
	got, err := store.FetchPendingActionsForMachine(context.Background(), 31, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].GUID != "only" || fmt.Sprint(got[0].Params) != fmt.Sprint(cmd) {
		t.Fatalf("got %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMarkActionDone_acquittement_passe_toutes_les_actions_regroupees_en_done(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewPortalStore(sqlx.NewDb(db, "sqlmock"))
	mock.ExpectQuery(`SELECT machine_id FROM cloud_actions WHERE guid = \$1`).
		WithArgs("last").
		WillReturnRows(sqlmock.NewRows([]string{"machine_id"}).AddRow(31))
	mock.ExpectExec(`UPDATE cloud_actions SET status = \$1, done_at = NOW\(\)\s+WHERE guid = \$2\s+OR \(machine_id = \$3 AND status = \$4\)`).
		WithArgs(domain.ActionStatusDone, "last", 31, domain.ActionStatusDelivered).
		WillReturnResult(sqlmock.NewResult(0, 14))
	if err := store.MarkActionDone(context.Background(), "last"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetLastCloudAction_renvoie_la_derniere_commande(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewPortalStore(sqlx.NewDb(db, "sqlmock"))
	raw, _ := json.Marshal(lightCmd(613, 64))
	mock.ExpectQuery(`FROM cloud_actions\s+WHERE user_id = \$1\s+ORDER BY created_at DESC\s+LIMIT 1`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"guid", "user_id", "machine_id", "params", "status", "created_at"}).
			AddRow("last", 1, 31, raw, domain.ActionStatusDone, time.Now()))
	a, err := store.GetLastCloudActionForUser(context.Background(), 1)
	if err != nil || a.GUID != "last" || a.Status != domain.ActionStatusDone {
		t.Fatalf("got %+v err=%v", a, err)
	}
}

// NR: NR-backend-9 essensys-hub/essensys-user-portal-backend#33
// Production 2026-10-07: "Tout allumer" enqueued 21 commands, the hub merged
// them keeping the last value per index and the cabinet received a command
// that lit a single lamp.
func Test_NR_backend_9_tout_allumer_livre_les_21_lampes(t *testing.T) {
	lights := []struct{ mask, on, off int }{
		{4, 616, 610}, {1, 611, 605}, {1, 613, 607}, {1, 616, 610}, {2, 616, 610}, {128, 615, 609},
		{8, 616, 610}, {16, 616, 610}, {128, 612, 606}, {64, 612, 606}, {1, 615, 609}, {128, 616, 610},
		{8, 615, 609}, {32, 615, 609}, {64, 615, 609}, {32, 612, 606}, {128, 614, 608}, {64, 614, 608},
		{32, 614, 608}, {16, 614, 608}, {8, 611, 605},
	}
	rows := make([]pendingRow, 0, len(lights))
	for i, l := range lights {
		rows = append(rows, pendingRow{fmt.Sprintf("g%02d", i), lightCmd(l.on, l.mask)})
	}
	// GET /api/myactions reads 20 pending actions per poll: the 21st command
	// comes with the next poll.
	first, mock1 := mockPending(t, 31, rows[:20])
	got, err := first.FetchPendingActionsForMachine(context.Background(), 31, 20)
	if err != nil {
		t.Fatal(err)
	}
	second, mock2 := mockPending(t, 31, rows[20:])
	next, err := second.FetchPendingActionsForMachine(context.Background(), 31, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(next) != 1 {
		t.Fatalf("each poll should deliver 1 merged action, got %d then %d", len(got), len(next))
	}
	v := valuesOf(domain.MergeBlockActions(got[0].Params, next[0].Params))
	for i, l := range lights {
		on, _ := strconv.Atoi(v[l.on])
		if on&l.mask == 0 {
			t.Errorf("lamp %d (index %d bit %d) not lit: %d=%q", i, l.on, l.mask, l.on, v[l.on])
		}
	}
	for _, m := range []sqlmock.Sqlmock{mock1, mock2} {
		if err := m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
