package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/handlers"
	"github.com/gujaratharva021-lgtm/ecommerce-backend/internal/middleware"
)

// SetupRoutes defines every API endpoint for the application.
func SetupRoutes(router *gin.Engine) {
	router.Use(middleware.CORS())

	router.GET("/health", handlers.HealthCheck)

	// Serve uploaded images (e.g. /uploads/169999.jpg)
	router.Static("/uploads", "./uploads")

	api := router.Group("/api/v1")
	{
		// ---- Auth routes (public) ----
		auth := api.Group("/auth")
		{
			// Rate-limited
			auth.POST("/send-otp", middleware.RateLimit(5, time.Minute), handlers.SendOTP)
			auth.POST("/verify-otp", middleware.RateLimit(10, time.Minute), handlers.VerifyOTP)
			auth.GET("/me", middleware.AuthMiddleware(), handlers.Me)
			auth.PUT("/me", middleware.AuthMiddleware(), handlers.UpdateProfile)
			auth.DELETE("/me", middleware.AuthMiddleware(), handlers.DeleteAccount)
		}

		// ---- Product routes (public) ----
		products := api.Group("/products")
		{
			products.GET("", handlers.GetProducts)        // ?search=&category_id=&min_price=&max_price=&in_stock=&sort=&page=&limit=
			products.GET("/search", handlers.GetProducts) // ?q=... (alias for /products?search=...) - registered before /:id so "search" is never parsed as a product ID
			products.GET("/:id", handlers.GetProductByID)

			products.GET("/:id/reviews", handlers.GetProductReviews)
			products.POST("/:id/reviews", middleware.AuthMiddleware(), handlers.UpsertReview)
			products.DELETE("/:id/reviews", middleware.AuthMiddleware(), handlers.DeleteReview)
		}

		// ---- Category routes (public) ----
		categories := api.Group("/categories")
		{
			categories.GET("", handlers.GetCategories)
			subcategories := api.Group("/subcategories")
			{
				subcategories.GET("", handlers.GetSubcategories)
			}
		}

		api.POST("/device-token", handlers.RegisterDeviceToken)

		// ---- Serviceability routes (public) ----
		api.GET("/serviceability", handlers.CheckServiceability)
		api.POST("/payments/razorpay-webhook", handlers.RazorpayWebhook)

		// ---- Notification routes (protected) ----
		api.GET("/notifications", middleware.AuthMiddleware(), handlers.GetMyNotifications)

		// ---- Wallet routes (protected) ----
		api.GET("/wallet", middleware.AuthMiddleware(), middleware.CustomerOnly(), handlers.GetWallet)
		api.POST("/wallet/topup", middleware.AuthMiddleware(), middleware.CustomerOnly(), handlers.CreateWalletTopupOrder)
		api.POST("/wallet/topup/verify", middleware.AuthMiddleware(), middleware.CustomerOnly(), handlers.VerifyWalletTopup)
		api.GET("/returns", middleware.AuthMiddleware(), handlers.GetMyReturns)

		// ---- Support ticket routes (protected, customer-facing) ----
		api.POST("/support/tickets", middleware.AuthMiddleware(), handlers.CreateTicket)
		api.GET("/support/tickets", middleware.AuthMiddleware(), handlers.GetMyTickets)
		api.GET("/support/tickets/:id/messages", middleware.AuthMiddleware(), handlers.GetTicketMessages)
		api.POST("/support/tickets/:id/messages", middleware.AuthMiddleware(), handlers.ReplyToTicket)
		api.GET("/support/orders/:id/window", middleware.AuthMiddleware(), handlers.GetOrderComplaintWindow)

		delivery := api.Group("/delivery")
		{
			delivery.POST("/send-otp", middleware.RateLimit(5, time.Minute), handlers.SendPartnerOTP)
			delivery.POST("/verify-otp", middleware.RateLimit(10, time.Minute), handlers.VerifyPartnerOTP)
			delivery.GET("/onboarding", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.GetOnboarding)
			delivery.GET("/stores", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.GetDeliveryStores)
			delivery.PUT("/onboarding/vehicle", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.SaveOnboardingVehicle)
			delivery.PUT("/onboarding/store", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.SaveOnboardingStore)
			delivery.PUT("/onboarding/documents", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.SaveOnboardingDocuments)
			delivery.PUT("/onboarding/selfie", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.SaveOnboardingSelfie)
			delivery.PUT("/onboarding/payout", middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.SaveOnboardingPayout)
			delivery.POST("/onboarding/upload", middleware.RateLimit(20, time.Minute), middleware.AuthMiddleware(), middleware.DeliveryPartnerAnyStatus(), handlers.UploadImage)
			delivery.PUT("/location", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateLocation)
			delivery.POST("/checkin", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.CheckInToStore)
			delivery.GET("/checkin", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyCheckin)
			delivery.GET("/status", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetDeliveryAvailability)
			delivery.PUT("/status", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateDeliveryAvailability)
			delivery.GET("/orders", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyDeliveries)
			delivery.PUT("/orders/:id/status", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateDeliveryOrderStatus)
			delivery.PUT("/orders/:id/deliver", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.ConfirmDelivery)
			delivery.GET("/profile", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetDeliveryProfile)
			delivery.GET("/rating", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyDeliveryRating)
			delivery.GET("/ratings", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyDeliveryRatings)
			delivery.GET("/orders/:id/rating", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetDeliveryOrderRating)
			delivery.PUT("/profile", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateDeliveryProfile)
			delivery.PUT("/profile/photo", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateDeliveryProfilePhoto)
			delivery.GET("/availability", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetDeliveryAvailability)
			delivery.PUT("/availability", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateDeliveryAvailability)
			delivery.PUT("/orders/:id/accept", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.AcceptAssignment)
			delivery.PUT("/orders/:id/reject", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.RejectAssignment)
			delivery.PUT("/orders/:id/delivery-status", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UpdateDeliveryStatus)

			delivery.PUT("/orders/:id/delivery-proof", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UploadDeliveryProof)
			delivery.PUT("/orders/:id/resolve-failed", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.ResolveFailedDelivery)
			delivery.GET("/earnings", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyEarnings)
			delivery.GET("/gigs", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyGigs)
			delivery.POST("/gigs/book", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.BookMyGigs)
			delivery.DELETE("/gigs/book", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.CancelMyGig)
			delivery.GET("/cod-summary", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyCODSummary)
			delivery.GET("/cod-settlements", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyCODSettlements)
			delivery.GET("/notifications", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyDeliveryNotifications)
			delivery.PUT("/notifications/:id/read", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.MarkDeliveryNotificationRead)
			delivery.PUT("/notifications/read-all", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.MarkAllDeliveryNotificationsRead)
			delivery.POST("/upload", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.UploadImage)
			delivery.GET("/returns", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.GetMyReturnPickups)
			delivery.PUT("/returns/:id/accept", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.AcceptReturnPickup)
			delivery.PUT("/returns/:id/reject", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.RejectReturnPickup)
			delivery.PUT("/returns/:id/en-route", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.MarkReturnPickupEnRoute)
			delivery.PUT("/returns/:id/arrived", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.MarkReturnPickupArrived)
			delivery.PUT("/returns/:id/picked-up", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.ConfirmReturnPickedUp)
			delivery.PUT("/returns/:id/handover", middleware.AuthMiddleware(), middleware.DeliveryPartnerOnly(), handlers.HandoverReturnToWarehouse)
		}

		warehouse := api.Group("/warehouse")
		{
			warehouse.POST("/send-otp", middleware.RateLimit(5, time.Minute), handlers.SendWarehouseStaffOTP)
			warehouse.POST("/verify-otp", middleware.RateLimit(10, time.Minute), handlers.VerifyWarehouseStaffOTP)

			warehouseStockTransfers := warehouse.Group("/stock-transfers")
			warehouseStockTransfers.Use(middleware.AuthMiddleware(), middleware.WarehouseStaffOnly())
			{
				warehouseStockTransfers.POST("", handlers.RequestStockTransfer)
				warehouseStockTransfers.GET("", handlers.GetMyStockTransfers)
				warehouseStockTransfers.PUT("/:id/receive", handlers.ReceiveStockTransfer)
				warehouseStockTransfers.PUT("/:id/approve", handlers.ApproveStockTransferByWarehouseStaff)
				warehouseStockTransfers.PUT("/:id/reject", handlers.RejectStockTransferByWarehouseStaff)
			}
			warehouseAuthed := warehouse.Group("")
			warehouseAuthed.Use(middleware.AuthMiddleware(), middleware.WarehouseStaffOnly(), middleware.InjectWarehouseScope())
			{
				warehouseAuthed.GET("/dashboard", handlers.GetWarehouseDashboard)
				warehouseAuthed.GET("/checkin-qr", handlers.GetStoreCheckinQR)
				warehouseAuthed.GET("/orders", handlers.GetWarehouseOrders)
				warehouseAuthed.PUT("/orders/:id/accept", handlers.AcceptOrder)
				warehouseAuthed.GET("/orders/:id/handover", handlers.GetHandover)
				warehouseAuthed.POST("/orders/:id/handover", handlers.HandoverOrder)
				warehouseAuthed.GET("/orders/:id/invoice", handlers.GetOrderInvoice)
				warehouseAuthed.GET("/picking/:id", handlers.GetPickingTask)
				warehouseAuthed.PUT("/picking/:id/start", handlers.StartPicking)
				warehouseAuthed.PUT("/picking/items/:itemId", handlers.MarkPickItem)
				warehouseAuthed.PUT("/picking/items/:itemId/scan", handlers.ScanPickItem)
				warehouseAuthed.PUT("/picking/:id/complete", handlers.CompletePicking)
				warehouseAuthed.GET("/packing/:id", handlers.GetPackingTask)
				warehouseAuthed.PUT("/packing/:id/start", handlers.StartPacking)
				warehouseAuthed.PUT("/packing/:id/complete", handlers.CompletePacking)
				warehouseAuthed.GET("/exceptions", handlers.GetWarehouseExceptions)
				warehouseAuthed.GET("/exceptions/:id", handlers.GetWarehouseException)
				warehouseAuthed.PUT("/exceptions/:id", handlers.UpdateWarehouseException)
				warehouseAuthed.GET("/staff/performance/me", handlers.GetMyPerformance)
				warehouseAuthed.GET("/staff/performance", handlers.GetWarehouseStaffPerformance)
				warehouseAuthed.GET("/staff", handlers.GetWarehouseStaffOverview)
				warehouseAuthed.GET("/zones", handlers.GetWarehouseZones)
				warehouseAuthed.POST("/zones", middleware.InventoryManagerOnly(), handlers.CreateWarehouseZone)
				warehouseAuthed.DELETE("/zones/:zoneId", middleware.InventoryManagerOnly(), handlers.DeleteZone)
				warehouseAuthed.GET("/zones/:zoneId/racks", handlers.GetZoneRacks)
				warehouseAuthed.POST("/zones/:zoneId/racks", middleware.InventoryManagerOnly(), handlers.CreateRack)
				warehouseAuthed.DELETE("/racks/:rackId", middleware.InventoryManagerOnly(), handlers.DeleteRack)
				warehouseAuthed.GET("/racks/:rackId/bins", handlers.GetRackBins)
				warehouseAuthed.POST("/racks/:rackId/bins", middleware.InventoryManagerOnly(), handlers.CreateBin)
				warehouseAuthed.DELETE("/bins/:binId", middleware.InventoryManagerOnly(), handlers.DeleteBin)
				warehouseAuthed.GET("/inventory", handlers.GetWarehouseInventory)
				warehouseAuthed.GET("/inventory/:productId", handlers.GetProductInventory)
				warehouseAuthed.POST("/inventory/:productId/adjust", middleware.InventoryManagerOnly(), handlers.AdjustStock)
				warehouseAuthed.GET("/stock-movements", handlers.GetStockMovements)
				warehouseAuthed.POST("/receiving", middleware.InventoryManagerOnly(), handlers.CreateReceiving)
				warehouseAuthed.GET("/receiving", handlers.GetWarehouseReceivings)
				warehouseAuthed.GET("/receiving/:id", handlers.GetReceiving)
				warehouseAuthed.PUT("/receiving/:id/receive", middleware.InventoryManagerOnly(), handlers.MarkReceiving)
				warehouseAuthed.PUT("/receiving/:id/qc", middleware.InventoryManagerOnly(), handlers.QCReceiving)
				warehouseAuthed.PUT("/receiving/:id/putaway", middleware.InventoryManagerOnly(), handlers.PutAwayReceiving)
				warehouseAuthed.POST("/batches", handlers.CreateBatch)
				warehouseAuthed.GET("/batches", handlers.GetWarehouseBatches)
				warehouseAuthed.GET("/batches/expiring", handlers.GetExpiringBatches)
				warehouseAuthed.PUT("/batches/:id/quantity", handlers.AdjustBatchQuantity)
				warehouseAuthed.DELETE("/batches/:id", handlers.DeleteBatch)
				warehouseAuthed.GET("/audit-logs", handlers.GetWarehouseAuditLogs)
				warehouseAuthed.GET("/notifications", handlers.GetWarehouseNotifications)
				warehouseAuthed.PUT("/notifications/:id/read", handlers.MarkNotificationRead)
				warehouseAuthed.PUT("/notifications/read-all", handlers.MarkAllNotificationsRead)
				warehouseAuthed.POST("/substitutions", handlers.CreateSubstitutionRequest)
				warehouseAuthed.GET("/substitutions", handlers.GetSubstitutionRequests)
				warehouseAuthed.GET("/substitutions/:id", handlers.GetSubstitutionRequest)
				warehouseAuthed.PUT("/substitutions/:id/approve", middleware.RequireWarehouseRole("warehouse_manager"), handlers.ApproveSubstitutionRequest)
				warehouseAuthed.PUT("/substitutions/:id/reject", middleware.RequireWarehouseRole("warehouse_manager"), handlers.RejectSubstitutionRequest)
				warehouseAuthed.GET("/returns", handlers.GetWarehouseReturns)
				warehouseAuthed.PUT("/returns/:id/approve", handlers.ApproveReturn)
				warehouseAuthed.PUT("/returns/:id/reject", handlers.RejectReturn)
				warehouseAuthed.PUT("/returns/:id/assign-pickup", handlers.AssignReturnPickup)
				warehouseAuthed.PUT("/returns/:id/confirm-received", handlers.ConfirmReturnReceived)
			}
		}

		// ---- Cart routes (protected) ----
		cart := api.Group("/cart")
		cart.Use(middleware.AuthMiddleware(), middleware.CustomerOnly())
		{
			cart.GET("", handlers.GetCart)
			cart.POST("", handlers.AddToCart)
			cart.PUT("/:item_id", handlers.UpdateCartItem)
			cart.DELETE("/:item_id", handlers.RemoveFromCart)
		}

		// ---- Address routes (protected) ----
		addresses := api.Group("/addresses")
		addresses.Use(middleware.AuthMiddleware(), middleware.CustomerOnly())
		{
			addresses.GET("", handlers.ListAddresses)
			addresses.POST("", handlers.CreateAddress)
			addresses.PUT("/:id", handlers.UpdateAddress)
			addresses.DELETE("/:id", handlers.DeleteAddress)
			addresses.PUT("/:id/default", handlers.SetDefaultAddress)
		}

		// ---- Wishlist routes (protected) ----
		wishlist := api.Group("/wishlist")
		wishlist.Use(middleware.AuthMiddleware(), middleware.CustomerOnly())
		{
			wishlist.GET("", handlers.GetWishlist)
			wishlist.POST("", handlers.AddToWishlist)
			wishlist.DELETE("/:product_id", handlers.RemoveFromWishlist)
		}

		// ---- Order routes (protected) ----
		orders := api.Group("/orders")
		orders.Use(middleware.AuthMiddleware(), middleware.CustomerOnly())
		{
			orders.POST("/checkout", handlers.Checkout)                    // body: { address_id?, payment_method?: "cod"|"online" }
			orders.GET("/checkout/estimate", handlers.GetCheckoutEstimate) // ?address_id= - preview items_amount/delivery_charge/platform_fee before placing the order
			orders.GET("", handlers.GetOrders)
			orders.GET("/:id", handlers.GetOrderByID)
			orders.GET("/:id/tracking", handlers.GetOrderTracking)
			orders.POST("/:id/rating", handlers.SubmitDeliveryRating)
			orders.GET("/:id/rating", handlers.GetDeliveryRatingForOrder)
			orders.PUT("/:id/cancel", handlers.CancelOrder)
			orders.POST("/:id/return", handlers.RequestReturn)
			orders.POST("/:id/payment", handlers.CreatePaymentOrder)   // creates Razorpay order (payment_method: online only)
			orders.POST("/:id/payment/verify", handlers.VerifyPayment) // verifies signature, marks order paid
			orders.GET("/:id/invoice/pdf", handlers.GetMyOrderInvoicePDF)
		}

		// ---- Coupon routes (protected) ----
		coupons := api.Group("/coupons")
		coupons.Use(middleware.AuthMiddleware(), middleware.CustomerOnly())
		{
			coupons.POST("/validate", handlers.ValidateCouponHandler)
		}

		// ---- Upload routes (protected, structure ready for product/category images) ----
		upload := api.Group("/upload")
		upload.Use(middleware.AuthMiddleware(), middleware.AdminOrCustomerOnly())
		{
			upload.POST("", handlers.UploadImage)
		}

		// ---- Admin routes (protected, admin role only) ----
		admin := api.Group("/admin")
		admin.Use(middleware.AuthMiddleware(), middleware.AdminOnly())
		admin.GET("/control-tower", handlers.GetControlTowerOverview)
		admin.PUT("/control-tower/platform-pause", handlers.UpdatePlatformPause)
		admin.PUT("/control-tower/city-status", handlers.UpdateCityStoreStatus)
		admin.GET("/control-tower/operations", handlers.GetLiveOperations)
		admin.GET("/control-tower/settings", handlers.GetControlTowerSettings)
		admin.PUT("/control-tower/settings/cod", handlers.UpdateCODEnabled)
		admin.GET("/delivery/unassigned-orders", handlers.GetUnassignedOrders)
		admin.GET("/delivery/stuck-orders", handlers.GetStuckOrders)
		admin.GET("/delivery/active", handlers.GetActiveDeliveries)
		admin.GET("/delivery/rider-workload", handlers.GetRiderWorkload)
		admin.GET("/delivery/failed", handlers.GetFailedDeliveries)
		admin.GET("/upi-collections", handlers.GetUPICollections)
		admin.PUT("/upi-collections/:id/review", handlers.ReviewUPICollection)

		adminProcurement := admin.Group("/procurement")
		{
			adminProcurement.GET("/purchase-orders", handlers.ListPurchaseOrders)
			adminProcurement.GET("/purchase-orders/:id", handlers.GetPurchaseOrder)
			adminProcurement.POST("/purchase-orders", handlers.CreatePurchaseOrder)
			adminProcurement.PUT("/purchase-orders/:id/status", handlers.UpdatePurchaseOrderStatus)
			adminProcurement.POST("/purchase-orders/:id/receive", handlers.ReceivePurchaseOrderItems)
			adminProcurement.GET("/replenishment", handlers.GetReplenishmentSuggestions)
		}
		admin.PUT("/products/:id/reorder-level", handlers.UpdateProductReorderLevel)

		adminSuppliers := admin.Group("/procurement/suppliers")
		{
			adminSuppliers.GET("", handlers.ListVendors)
			adminSuppliers.POST("", handlers.CreateVendor)
			adminSuppliers.PUT("/:id", handlers.UpdateVendor)
			adminSuppliers.DELETE("/:id", handlers.DeleteVendor)
		}
		{
			adminFinance := admin.Group("/finance")
			adminFinance.Use(middleware.FinanceOnly())
			{
				adminFinance.GET("/dashboard", handlers.FinanceDashboard)
				adminFinance.GET("/revenue", handlers.GetRevenueSummary)
				adminFinance.GET("/mismatch-center", handlers.ListMismatches)
				adminFinance.POST("/mismatch-center/dismiss", handlers.DismissMismatch)
				adminFinance.POST("/mismatch-center/undismiss", handlers.UndismissMismatch)
				adminFinance.GET("/profit-loss", handlers.GetProfitLoss)
				adminFinance.GET("/gst", handlers.GetGSTSummary)
				adminFinance.GET("/mis", handlers.GetWeeklyMIS)
				adminFinance.GET("/mis/export", handlers.ExportWeeklyMIS)
				adminFinance.GET("/mis/monthly", handlers.GetMonthlyMIS)
				adminFinance.GET("/mis/monthly/export", handlers.ExportMonthlyMIS)
				adminFinance.GET("/mis/weekly-template", handlers.GetWeeklyTemplateMIS)
				adminFinance.GET("/mis/weekly-template/export", handlers.ExportWeeklyTemplateMIS)
				adminFinance.POST("/mis/manual-entry", handlers.UpsertMISManualEntry)
				adminFinance.DELETE("/mis/manual-entry/:id", handlers.DeleteMISManualEntry)
				adminFinance.PUT("/mis/expense-approval/:id", handlers.UpdateMISExpenseApproval)
				adminVendors := adminFinance.Group("/vendors")
				{
					adminVendors.GET("", handlers.ListVendors)
					adminVendors.POST("", handlers.CreateVendor)
					adminVendors.PUT("/:id", handlers.UpdateVendor)
					adminVendors.POST("/:id/bank-change-request", handlers.RequestVendorBankChange)
					adminVendors.DELETE("/:id", handlers.DeleteVendor)
				}
				adminVendorBills := adminFinance.Group("/vendor-bills")
				{
					adminVendorBills.GET("", handlers.ListVendorBills)
					adminVendorBills.POST("", handlers.CreateVendorBill)
					adminVendorBills.POST("/:id/pay", handlers.PayVendorBill)
					adminVendorBills.POST("/:id/void", handlers.VoidVendorBill)
					adminVendorBills.POST("/:id/hold", handlers.HoldVendorBill)
					adminVendorBills.POST("/:id/dispute", handlers.DisputeVendorBill)
					adminVendorBills.POST("/:id/release-hold", handlers.ReleaseHoldVendorBill)
					adminVendorBills.POST("/:id/debit-note", handlers.CreateDebitNote)
				}
				adminVendorBankChanges := adminFinance.Group("/vendor-bank-change-requests")
				{
					adminVendorBankChanges.GET("", handlers.ListVendorBankChangeRequests)
					adminVendorBankChanges.POST("/:id/approve", handlers.ApproveVendorBankChange)
					adminVendorBankChanges.POST("/:id/reject", handlers.RejectVendorBankChange)
				}
				adminAccounts := adminFinance.Group("/accounts")
				{
					adminAccounts.GET("", handlers.ListAccounts)
					adminAccounts.POST("", handlers.CreateAccount)
					adminAccounts.PUT("/:id", handlers.UpdateAccount)
				}
				adminLedger := adminFinance.Group("/ledger")
				{
					adminLedger.GET("", handlers.ListLedgerEntries)
					adminLedger.POST("", handlers.CreateManualJournalEntry)
					adminLedger.GET("/pending", handlers.ListPendingJournalEntries)
					adminLedger.POST("/pending/:id/approve", handlers.ApprovePendingJournalEntry)
					adminLedger.POST("/pending/:id/reject", handlers.RejectPendingJournalEntry)
					adminLedger.GET("/trial-balance", handlers.GetTrialBalance)
					adminCreditNotes := adminFinance.Group("/credit-notes")
					{
						adminCreditNotes.GET("", handlers.ListCreditNotes)
						adminCreditNotes.GET("/:id", handlers.GetCreditNote)
					}
					adminDebitNotes := adminFinance.Group("/debit-notes")
					{
						adminDebitNotes.GET("", handlers.ListDebitNotes)
						adminDebitNotes.GET("/:id", handlers.GetDebitNote)
					}
					adminRiderCODDeposits := adminFinance.Group("/rider-cod-deposits")
					{
						adminRiderCODDeposits.GET("", handlers.ListRiderCODDeposits)
						adminRiderCODDeposits.POST("", handlers.CreateRiderCODDeposit)
						adminRiderCODDeposits.POST("/:id/verify", handlers.VerifyRiderCODDeposit)
					}
					adminRiderPayouts := adminFinance.Group("/rider-payouts")
					{
						adminRiderPayouts.GET("", handlers.ListRiderPayouts)
						adminRiderPayouts.POST("", handlers.CreateRiderPayout)
						adminRiderPayouts.POST("/:id/approve", handlers.ApproveRiderPayout)
						adminRiderPayouts.POST("/:id/pay", handlers.PayRiderPayout)
					}
				}
				adminBankTransactions := adminFinance.Group("/bank-transactions")
				{
					adminBankTransactions.GET("", handlers.ListBankTransactions)
					adminBankTransactions.POST("", handlers.CreateBankTransaction)
					adminBankTransactions.POST("/:id/match", handlers.MatchBankTransaction)
					adminBankTransactions.POST("/:id/ignore", handlers.IgnoreBankTransaction)
					adminBankTransactions.POST("/:id/void", handlers.VoidBankTransaction)
				}
				adminFinance.GET("/expenses", handlers.ListExpenses)
				adminFinance.POST("/expenses", handlers.CreateExpense)
				adminFinance.PUT("/expenses/:id", handlers.UpdateExpense)
				adminFinance.POST("/expenses/:id/submit", handlers.SubmitExpense)
				adminFinance.POST("/expenses/:id/approve", handlers.ApproveExpense)
				adminFinance.POST("/expenses/:id/reject", handlers.RejectExpense)
				adminFinance.POST("/expenses/:id/pay", handlers.PayExpense)
				adminFinance.DELETE("/expenses/:id", handlers.DeleteExpense)
				adminFinance.GET("/payroll", handlers.ListPayroll)
				adminFinance.POST("/payroll", handlers.CreatePayroll)
				adminFinance.PUT("/payroll/:id", handlers.UpdatePayroll)
				adminFinance.DELETE("/payroll/:id", handlers.DeletePayroll)
			}

			adminCategories := admin.Group("/categories")
			{
				adminCategories.POST("", handlers.CreateCategory)
				adminCategories.PUT("/:id", handlers.UpdateCategory)
				adminCategories.DELETE("/:id", handlers.DeleteCategory)

				adminSubcategories := admin.Group("/subcategories")
				{
					adminSubcategories.POST("", handlers.CreateSubcategory)
					adminSubcategories.PUT("/:id", handlers.UpdateSubcategory)
					adminSubcategories.DELETE("/:id", handlers.DeleteSubcategory)
				}
			}

			adminProducts := admin.Group("/products")
			{
				adminProducts.POST("", handlers.CreateProduct)
				adminProducts.PUT("/:id", handlers.UpdateProduct)
				adminProducts.DELETE("/:id", handlers.DeleteProduct)
				adminProducts.PUT("/:id/inventory", handlers.UpdateInventory)
				adminProducts.POST("/bulk-cost-price", handlers.BulkUpdateCostPrice)
			}

			adminOrders := admin.Group("/orders")
			{
				adminOrders.GET("", handlers.GetAllOrders) // ?status=&page=&limit=
				adminOrders.PUT("/:id/status", handlers.UpdateOrderStatus)
				adminOrders.GET("/:id/tracking", handlers.GetOrderTrackingAdmin)
			}

			adminReturns := admin.Group("/returns")
			{
				adminReturns.GET("", handlers.GetReturns)
				adminReturns.PUT("/:id/approve", handlers.ApproveReturn)
				adminReturns.PUT("/:id/reject", handlers.RejectReturn)
				adminReturns.PUT("/:id/assign-pickup", handlers.AssignReturnPickup)
			}

			adminCoupons := admin.Group("/coupons")
			{
				adminCoupons.POST("", handlers.CreateCoupon)
				adminCoupons.GET("", handlers.GetCoupons)
				adminCoupons.PUT("/:id/status", handlers.UpdateCouponStatus)
			}
			adminAnalytics := admin.Group("/analytics")
			{
				adminAnalytics.GET("/summary", handlers.GetAnalyticsSummary)
				adminAnalytics.GET("/products", handlers.GetProductPerformance)
				adminAnalytics.GET("/picker-performance", handlers.GetAdminPickerPerformance)
				adminAnalytics.GET("/dashboard", handlers.GetDashboardOverview)
			}

			adminDeliveryPartners := admin.Group("/delivery-partners")
			{
				adminDeliveryPartners.POST("", handlers.CreateDeliveryPartner)
				adminDeliveryPartners.GET("", handlers.GetDeliveryPartners)
				adminDeliveryPartners.PUT("/:id", handlers.UpdateDeliveryPartner)
				adminDeliveryPartners.DELETE("/:id", handlers.DeleteDeliveryPartner)
				adminDeliveryPartners.GET("/:id/onboarding", handlers.GetPartnerOnboardingAdmin)
				adminDeliveryPartners.PUT("/:id/onboarding/review", handlers.ReviewPartnerOnboarding)
				adminDeliveryPartners.GET("/:id/location", handlers.GetDeliveryPartnerLocation)
			}

			admin.PUT("/orders/:id/assign-delivery", handlers.AssignDeliveryPartner)

			adminWarehouses := admin.Group("/warehouses")
			{
				adminWarehouses.POST("", handlers.CreateWarehouse)
				adminWarehouses.GET("", handlers.GetWarehouses)
				adminWarehouses.GET("/:id", handlers.GetWarehouse)
				adminWarehouses.PUT("/:id", handlers.UpdateWarehouse)
				adminWarehouses.DELETE("/:id", handlers.DeleteWarehouse)
				adminWarehouses.PUT("/:id/service-area", handlers.SetWarehouseServiceArea)

				adminWarehouseStaff := admin.Group("/warehouse-staff")
				{
					adminWarehouseStaff.POST("", handlers.CreateWarehouseStaff)
					adminWarehouseStaff.GET("", handlers.GetWarehouseStaff)
					adminWarehouseStaff.PUT("/:id", handlers.UpdateWarehouseStaff)
					adminWarehouseStaff.DELETE("/:id", handlers.DeleteWarehouseStaff)
				}

				adminStockTransfers := admin.Group("/stock-transfers")
				{
					adminStockTransfers.GET("", handlers.GetStockTransfers)
					adminStockTransfers.PUT("/:id/approve", handlers.ApproveStockTransfer)
					adminStockTransfers.PUT("/:id/reject", handlers.RejectStockTransfer)
				}

				admin.POST("/wallet/credit/:user_id", handlers.AdminCreditWallet)

				admin.POST("/products/:id/barcode", handlers.GenerateProductBarcode)
				admin.DELETE("/coupons/:id", handlers.DeleteCoupon)
				admin.PUT("/stock-transfers/:id/cancel", handlers.CancelStockTransfer)

				adminCustomers := admin.Group("/customers")
				{
					adminCustomers.GET("", handlers.GetCustomers)
					adminCustomers.GET("/:id", handlers.GetCustomerByID)
					adminCustomers.PUT("/:id/block", handlers.BlockCustomer)
					adminCustomers.PUT("/:id/unblock", handlers.UnblockCustomer)
				}

				admin.GET("/inventory", handlers.GetInventoryOverview)

				adminStaff := admin.Group("/staff")
				{
					adminStaff.GET("", handlers.GetAdminStaff)
					adminStaff.PUT("/:id/role", handlers.UpdateStaffRole)
				}

				adminSettings := admin.Group("/settings")
				{
					adminSettings.GET("", handlers.GetSettings)
					adminSettings.PUT("", handlers.UpdateSettings)
				}

				admin.GET("/audit-logs", handlers.GetAuditLogs)
				admin.POST("/notifications/broadcast", handlers.BroadcastNotification)

				adminOffers := admin.Group("/offers")
				{
					adminOffers.GET("", handlers.GetOffers)
					adminOffers.POST("", handlers.CreateOffer)
					adminOffers.PUT("/:id/status", handlers.UpdateOfferStatus)
					adminOffers.DELETE("/:id", handlers.DeleteOffer)
				}

				adminBanners := admin.Group("/banners")
				{
					adminBanners.GET("", handlers.GetBanners)
					adminBanners.POST("", handlers.CreateBanner)
					adminBanners.PUT("/:id", handlers.UpdateBanner)
					adminBanners.DELETE("/:id", handlers.DeleteBanner)
				}

				adminDeliveryZones := admin.Group("/delivery-zones")
				{
					adminDeliveryZones.GET("", handlers.GetDeliveryZones)
					adminDeliveryZones.POST("", handlers.CreateDeliveryZone)
					adminDeliveryZones.PUT("/:id", handlers.UpdateDeliveryZone)
					adminDeliveryZones.DELETE("/:id", handlers.DeleteDeliveryZone)
				}

				adminSupportTickets := admin.Group("/support/tickets")
				{
					adminSupportTickets.GET("", handlers.GetAllTickets)
					adminSupportTickets.GET("/:id/messages", handlers.GetTicketMessagesAdmin)
					adminSupportTickets.POST("/:id/messages", handlers.AdminReplyToTicket)
					adminSupportTickets.PUT("/:id/status", handlers.UpdateTicketStatus)
					adminSupportTickets.PUT("/:id", handlers.UpdateTicket)
				}

				adminPayments := admin.Group("/payments")
				{
					adminPayments.GET("", handlers.GetAdminPayments)
					adminPayments.GET("/reconciliation", handlers.GetAdminPaymentReconciliation)
					adminPayments.GET("/:order_id", handlers.GetAdminPaymentDetail)
					adminPayments.PUT("/:order_id/status", handlers.UpdateAdminPaymentStatus)
					adminPayments.POST("/:id/settle-gateway", handlers.SettleGatewayPayment)
				}

				adminInvoices := admin.Group("/invoices")
				{
					adminInvoices.GET("", handlers.SearchInvoices)
					adminInvoices.GET("/:id", handlers.GetAdminInvoiceByID)
					adminInvoices.GET("/:id/pdf", handlers.GetAdminInvoicePDF)
				}

				adminReports := admin.Group("/reports")
				{
					adminReports.GET("/daily-sales", handlers.GetDailySalesReport)
					adminReports.GET("/daily-sales/export", handlers.ExportDailySalesReport)
					adminReports.GET("/range-sales", handlers.GetRangeSalesReport)
					adminReports.GET("/range-sales/export", handlers.ExportRangeSalesReport)
					adminReports.GET("/sales-register", handlers.GetSalesRegister)
					adminReports.GET("/purchase-register", handlers.GetPurchaseRegister)
					adminReports.GET("/rider-payable", handlers.GetRiderPayableReport)
					adminReports.GET("/gateway-settlement", handlers.GetGatewaySettlementReport)
					adminReports.GET("/cash-flow", handlers.GetCashFlowReport)
					adminReports.GET("/balance-sheet", handlers.GetBalanceSheet)
					adminReports.GET("/operations", handlers.GetOperationsReport)
				}
			}
		}
	}
}
