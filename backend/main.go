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

	// Chat WebSocket (token via query; handler validates and routes to a room)
	api.GET("/ws/chat", handlers.ChatWS)

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
		admin.GET("/personal-sections", handlers.ListPersonalSections)
		admin.POST("/personal-sections", handlers.CreatePersonalSection)
		admin.PUT("/sections/:id", handlers.UpdateSection)
		admin.PUT("/sections/:id/order", handlers.UpdateSectionOrder)
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

		// Drinks (preferred drinks list)
		admin.GET("/drinks", handlers.ListDrinks)
		admin.POST("/drinks", handlers.CreateDrink)
		admin.PUT("/drinks/:id", handlers.UpdateDrink)
		admin.DELETE("/drinks/:id", handlers.DeleteDrink)
		admin.GET("/drink-comments", handlers.ListDrinkComments)
		admin.GET("/stats/drinks", handlers.StatsDrinks)

		// Stats
		admin.GET("/stats/visits", handlers.StatsVisits)
		admin.GET("/stats/cottage", handlers.StatsCottage)
		admin.DELETE("/stats/cottage/:guestId", handlers.ResetCottageResponse)
		admin.GET("/stats/tournament", handlers.StatsTournament)
		admin.GET("/stats/loft", handlers.StatsLoft)
		admin.GET("/stats/attendance", handlers.StatsAttendance)

		// Comments
		admin.GET("/comments", handlers.ListComments)
		admin.PUT("/comments/:guestId/reply", handlers.ReplyComment)

		// Chat (admin views a chosen room)
		admin.GET("/chat/messages", handlers.GetAdminChatMessages)
		admin.DELETE("/chat/messages", handlers.ClearChat)
		admin.DELETE("/chat/messages/:id", handlers.DeleteChatMessage)
	}

	// Guest routes (authenticated guests)
	guest := api.Group("/guest", middleware.Auth(), middleware.RequireRole("friends", "family"))
	{
		guest.GET("/me", handlers.GetGuestInfo)
		guest.PUT("/response/friend", handlers.SaveFriendResponse)
		guest.PUT("/response/family", handlers.SaveFamilyResponse)
		guest.PUT("/response/attendance", handlers.SaveAttendance)
		guest.GET("/gifts", handlers.GetGifts)
		guest.POST("/gifts/:id/pick", handlers.PickGift)
		guest.DELETE("/gifts/:id/pick", handlers.UnpickGift)
		guest.PUT("/group-gift-pick", handlers.SaveGroupGiftPick)
		guest.GET("/group-gift-picks", handlers.GetGroupGiftPicks)
		guest.GET("/music", handlers.GetMusic)
		guest.GET("/friends", handlers.GetFriends)

		// Chat (guest's own room)
		guest.GET("/chat/messages", handlers.GetChatMessages)
		guest.GET("/chat/unread", handlers.GetChatUnread)
		guest.POST("/chat/seen", handlers.MarkChatSeen)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on :%s", port)
	r.Run(":" + port)
}
