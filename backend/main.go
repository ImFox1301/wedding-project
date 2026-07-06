package main

import (
	"log"
	"os"
	"wedding/db"
	"wedding/handlers"
	"wedding/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	db.Init()

	r := gin.Default()

	// CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Serve uploads
	uploadDir := "/app/uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		uploadDir = "uploads"
		os.MkdirAll(uploadDir, 0755)
	}
	r.Static("/uploads", uploadDir)

	api := r.Group("/api")

	// Auth
	api.POST("/auth/login", handlers.Login)
	api.GET("/auth/me", middleware.Auth(), handlers.Me)

	// Public invite page (by token, no auth needed)
	api.GET("/invite/:token", handlers.GetInvitePage)

	// Admin routes
	admin := api.Group("/admin", middleware.Auth(), middleware.AdminOnly())
	{
		// Guests
		admin.GET("/guests", handlers.ListGuests)
		admin.POST("/guests", handlers.CreateGuest)
		admin.GET("/guests/:id", handlers.GetGuest)
		admin.PUT("/guests/:id", handlers.UpdateGuest)
		admin.DELETE("/guests/:id", handlers.DeleteGuest)

		// Groups
		admin.GET("/groups", handlers.ListGroups)
		admin.POST("/groups", handlers.CreateGroup)
		admin.GET("/groups/:id", handlers.GetGroup)
		admin.PUT("/groups/:id", handlers.UpdateGroup)
		admin.DELETE("/groups/:id", handlers.DeleteGroup)

		// Invitation links
		admin.GET("/links", handlers.ListLinks)
		admin.GET("/links/available", handlers.AvailableForLink)
		admin.POST("/links", handlers.CreateLink)
		admin.DELETE("/links/:id", handlers.DeleteLink)

		// Gifts
		admin.GET("/gifts", handlers.ListGifts)
		admin.POST("/gifts", handlers.CreateGift)
		admin.PUT("/gifts/:id", handlers.UpdateGift)
		admin.DELETE("/gifts/:id", handlers.DeleteGift)
		admin.POST("/gifts/:id/photo", handlers.UploadGiftPhoto)
		admin.DELETE("/gifts/:id/photo", handlers.DeleteGiftPhoto)

		// Page sections
		admin.GET("/sections", handlers.ListSections)
		admin.POST("/sections", handlers.CreateSection)
		admin.PUT("/sections/:id", handlers.UpdateSection)
		admin.DELETE("/sections/:id", handlers.DeleteSection)
		admin.POST("/sections/:id/photos", handlers.UploadSectionPhoto)
		admin.DELETE("/sections/:id/photos/:photoId", handlers.DeleteSectionPhoto)

		// Settings
		admin.GET("/settings", handlers.GetSettings)
		admin.PUT("/settings", handlers.UpdateSettings)

		// Music
		admin.GET("/music", handlers.ListMusic)
		admin.POST("/music", handlers.UploadMusic)
		admin.DELETE("/music/:id", handlers.DeleteMusic)
		admin.PUT("/music/:id/order", handlers.UpdateMusicOrder)

		// Stats
		admin.GET("/stats/visits", handlers.StatsVisits)
		admin.GET("/stats/cottage", handlers.StatsCottage)
		admin.GET("/stats/tournament", handlers.StatsTournament)
		admin.GET("/stats/loft", handlers.StatsLoft)
	}

	// Guest routes (authenticated guests)
	guest := api.Group("/guest", middleware.Auth(), middleware.RequireRole("friends", "family"))
	{
		guest.GET("/me", handlers.GetGuestInfo)
		guest.PUT("/response/friend", handlers.SaveFriendResponse)
		guest.PUT("/response/family", handlers.SaveFamilyResponse)
		guest.GET("/gifts", handlers.GetGifts)
		guest.POST("/gifts/:id/pick", handlers.PickGift)
		guest.DELETE("/gifts/:id/pick", handlers.UnpickGift)
		guest.GET("/music", handlers.GetMusic)
		guest.GET("/friends", handlers.GetFriends)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	r.Run(":" + port)
}
