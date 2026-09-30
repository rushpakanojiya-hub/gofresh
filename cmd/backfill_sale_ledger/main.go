package main

// One-time backfill for orders whose sale was never posted to the ledger
// (see Mismatch Center check "sale_not_posted"). Reuses the exact same
// services.PostSalesLedgerEntry function that runs for new orders, so
// backfilled entries follow identical accounting logic - and since that
// function is already idempotent (checks reference_type="sale" AND
// reference_id=orderID before inserting), this script is safe to re-run.
//
// GROUP-B MODE (-generate-invoices): for orders that also have no invoice
// yet (the ~39 "genuine, payment-verified" orders from the Group A/B
// classification, order #346 excluded pending manual review of its
// missing gateway payment ID), this also calls the existing, unmodified
// services.GenerateInvoiceIfNotExists - reusing its tested GST/discount
// calculation rather than reimplementing it - and then backdates the
// invoice's generated_at to the order's created_at so the sale lands in
// the correct historical GST period instead of today's, before posting
// the ledger entry (which reads EntryDate from invoice.GeneratedAt).
// invoice.go itself is never modified, so live order flow is untouched.
//
// Usage (from the repo root, e.g. /home/ubuntu/ecommerce-backend on the
// server, or wherever your .env / DB config lives):
//
//   go run ./cmd/backfill_sale_ledger                      # dry run - lists sale-ledger candidates only
//   go run ./cmd/backfill_sale_ledger -apply                # posts ledger entries for orders that already have invoices
//   go run ./cmd/backfill_sale_ledger -generate-invoices     # dry run for the Group-B (invoice+ledger) list
//   go run ./cmd/backfill_sale_ledger -generate-invoices -apply  # generates backdated invoices + posts ledger for Group B
//
// Place this file at cmd/backfill_sale_ledger/main.go in the backend repo
// (create the directory) so it can import the internal packages the same
// way cmd/api/main.go does.

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/config"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/database"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/services"
)

type candidateOrder struct {
	ID            uint
	Status        string
	PaymentMethod string
	PaymentStatus string
	TotalAmount   float64
	CreatedAt     time.Time
}

// groupBOrderIDs is the manually-reviewed list of "genuine, payment-verified"
// orders from the Group A (seed/test) vs Group B (real) classification.
// Order #346 is deliberately excluded - its razorpay_payment_id is blank
// despite payment_status=paid, unlike every other online order in this
// set, and needs manual review before being treated as revenue.
var groupBOrderIDs = []uint{
	3, 6, 8, 9, 10, 11, 12, 13, 14,
	308, 309, 311, 313, 314, 315, 316, 317, 318, 319, 320, 321, 325, 327,
	328, 330, 331, 332, 333, 334, 335, 336, 338, 339, 340, 342, 344, 345, 347, 349,
}

func main() {
	apply := flag.Bool("apply", false, "actually write data (default: dry run, lists candidates only)")
	generateInvoices := flag.Bool("generate-invoices", false, "run the Group-B mode: generate backdated invoices + post ledger for the reviewed real-order list, instead of the plain sale-ledger backfill")
	flag.Parse()

	cfg := config.LoadConfig()
	database.ConnectDatabase(cfg)

	if *generateInvoices {
		runGroupB(*apply)
		return
	}

	var candidates []candidateOrder
	err := database.DB.Table("orders o").
		Select("o.id, o.status, o.payment_method, o.payment_status, o.total_amount, o.created_at").
		Where(`
			(
			  (o.payment_method = 'online' AND o.payment_status = 'paid')
			  OR (o.payment_method = 'cod' AND o.status = 'delivered')
			)
			AND o.status != 'cancelled'
			AND NOT EXISTS (
			  SELECT 1 FROM ledger_entries le
			  WHERE le.reference_type = 'sale' AND le.reference_id = o.id
			)
		`).
		Order("o.created_at ASC").
		Scan(&candidates).Error
	if err != nil {
		log.Fatalf("failed to load candidate orders: %v", err)
	}

	fmt.Printf("Found %d order(s) missing a sale ledger entry (cancelled orders excluded).\n\n", len(candidates))

	if !*apply {
		fmt.Println("DRY RUN - no ledger entries will be posted. Re-run with -apply to actually post them.")
		fmt.Println()
		var total float64
		for _, o := range candidates {
			fmt.Printf("  order #%-5d  status=%-10s  payment_method=%-6s  payment_status=%-8s  total=%.2f\n",
				o.ID, o.Status, o.PaymentMethod, o.PaymentStatus, o.TotalAmount)
			total += o.TotalAmount
		}
		fmt.Printf("\nTotal candidate revenue to be posted: %.2f\n", total)
		os.Exit(0)
	}

	fmt.Println("APPLY MODE - posting ledger entries now...")
	var posted, skipped, failed int
	for _, o := range candidates {
		if err := services.PostSalesLedgerEntry(o.ID); err != nil {
			fmt.Printf("  order #%-5d  FAILED: %v\n", o.ID, err)
			failed++
			continue
		}
		posted++
	}
	fmt.Printf("\nDone. Posted: %d, Skipped (already had entry): %d, Failed: %d\n", posted, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func runGroupB(apply bool) {
	var orders []candidateOrder
	err := database.DB.Table("orders o").
		Select("o.id, o.status, o.payment_method, o.payment_status, o.total_amount, o.created_at").
		Where("o.id IN ?", groupBOrderIDs).
		Order("o.created_at ASC").
		Scan(&orders).Error
	if err != nil {
		log.Fatalf("failed to load group-B orders: %v", err)
	}

	fmt.Printf("Group B: %d reviewed real orders (order #346 excluded pending manual review).\n\n", len(orders))

	if !apply {
		fmt.Println("DRY RUN - no invoices or ledger entries will be written. Re-run with -generate-invoices -apply to commit.")
		fmt.Println()
		var total float64
		for _, o := range orders {
			fmt.Printf("  order #%-5d  created=%s  status=%-10s  payment_method=%-6s  total=%.2f\n",
				o.ID, o.CreatedAt.Format("2006-01-02"), o.Status, o.PaymentMethod, o.TotalAmount)
			total += o.TotalAmount
		}
		fmt.Printf("\nTotal candidate revenue: %.2f\n", total)
		os.Exit(0)
	}

	fmt.Println("APPLY MODE - generating backdated invoices and posting ledger entries...")
	var invoiced, ledgerPosted, failed int
	for _, o := range orders {
		invoice, err := services.GenerateInvoiceIfNotExists(o.ID)
		if err != nil {
			fmt.Printf("  order #%-5d  INVOICE FAILED: %v\n", o.ID, err)
			failed++
			continue
		}
		invoiced++

		// Backdate to the order's original date so this lands in the
		// correct historical GST period instead of today's - invoice.go
		// itself is untouched, this is a targeted correction applied
		// only by this backfill script, only to rows it just created
		// (or that already existed for this order).
		if err := database.DB.Model(invoice).
			Where("id = ?", invoice.ID).
			Update("generated_at", o.CreatedAt).Error; err != nil {
			fmt.Printf("  order #%-5d  BACKDATE FAILED: %v\n", o.ID, err)
			failed++
			continue
		}

		if err := services.PostSalesLedgerEntry(o.ID); err != nil {
			fmt.Printf("  order #%-5d  LEDGER FAILED: %v\n", o.ID, err)
			failed++
			continue
		}
		ledgerPosted++
	}
	fmt.Printf("\nDone. Invoices created/found: %d, Ledger entries posted: %d, Failed: %d\n", invoiced, ledgerPosted, failed)
	if failed > 0 {
		os.Exit(1)
	}
}
