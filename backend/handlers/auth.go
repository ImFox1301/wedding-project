package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"time"
	"wedding/db"
	"wedding/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check admin credentials from env
	adminLogin := os.Getenv("ADMIN_LOGIN")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if req.Login == adminLogin && req.Password == adminPassword {
		token := makeToken(0, adminLogin, "admin")
		c.JSON(http.StatusOK, gin.H{"token": token, "role": "admin"})
		return
	}

	// Check guest in DB
	var (
		id           int
		passwordHash string
		role         string
	)
	err := db.DB.QueryRow(
		`SELECT id, password_hash, role FROM guests WHERE login=$1`, req.Login,
	).Scan(&id, &passwordHash, &role)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный логин или пароль"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный логин или пароль"})
		return
	}

	token := makeToken(id, req.Login, role)
	c.JSON(http.StatusOK, gin.H{"token": token, "role": role, "guest_id": id})
}

func makeToken(userID int, login, role string) string {
	claims := middleware.Claims{
		UserID: userID,
		Login:  login,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(middleware.JWTSecret())
	return signed
}

func Me(c *gin.Context) {
	role, _ := c.Get("role")
	login, _ := c.Get("login")
	userID, _ := c.Get("user_id")
	c.JSON(http.StatusOK, gin.H{"role": role, "login": login, "user_id": userID})
}
