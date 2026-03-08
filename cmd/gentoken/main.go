package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"converge-finance.com/m/internal/platform/auth"
)

func main() {
	userID := flag.String("user", "01HQXYZ123456789ABCDEF", "User ID (ULID)")
	entityID := flag.String("entity", "01JNQVXG7R0001ENTITY00001", "Entity ID (ULID)")
	roles := flag.String("roles", "admin", "Comma-separated roles (admin,accountant,controller,ap_clerk,ar_clerk,viewer)")
	expiry := flag.Duration("expiry", 24*time.Hour, "Token expiry duration")
	secret := flag.String("secret", "dev-secret-do-not-use-in-production", "JWT secret key")
	format := flag.String("format", "text", "Output format: text or json")

	flag.Parse()

	roleList := strings.Split(*roles, ",")
	for i := range roleList {
		roleList[i] = strings.TrimSpace(roleList[i])
	}

	jwtService := auth.NewJWTService(*secret, *expiry, 7*24*time.Hour)

	tokenPair, err := jwtService.GenerateTokenPair(*userID, *entityID, roleList)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating token: %v\n", err)
		os.Exit(1)
	}

	if *format == "json" {
		output, _ := json.MarshalIndent(tokenPair, "", "  ")
		fmt.Println(string(output))
	} else {
		fmt.Println("=== Development JWT Token ===")
		fmt.Printf("User ID:    %s\n", *userID)
		fmt.Printf("Entity ID:  %s\n", *entityID)
		fmt.Printf("Roles:      %s\n", strings.Join(roleList, ", "))
		fmt.Printf("Expires:    %s\n", tokenPair.ExpiresAt.Format(time.RFC3339))
		fmt.Println()
		fmt.Println("Access Token:")
		fmt.Println(tokenPair.AccessToken)
		fmt.Println()
		fmt.Println("Refresh Token:")
		fmt.Println(tokenPair.RefreshToken)
		fmt.Println()
		fmt.Println("Use in requests:")
		fmt.Printf("  Authorization: Bearer %s\n", tokenPair.AccessToken)
		fmt.Printf("  X-Entity-ID: %s\n", *entityID)
	}
}
