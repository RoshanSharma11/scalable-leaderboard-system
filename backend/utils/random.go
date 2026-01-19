package utils

import (
	"fmt"
	"math/rand"
	"time"
)

var rng *rand.Rand

func init() {
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))
}

// GenerateRandomUsername generates a random username with potential collisions
func GenerateRandomUsername(id int) string {
	firstNames := []string{
		"rahul", "priya", "amit", "sneha", "vijay", "anita", "rohan", "kavya",
		"arjun", "neha", "karan", "pooja", "aditya", "divya", "siddharth", "isha",
		"nikhil", "ritu", "varun", "megha", "akash", "shreya", "manish", "nisha",
		"rajesh", "swati", "deepak", "anjali", "suresh", "preeti",
	}

	lastNames := []string{
		"kumar", "sharma", "patel", "singh", "reddy", "gupta", "verma", "joshi",
		"mehta", "agarwal", "rao", "nair", "chopra", "khan", "das", "malhotra",
	}

	pattern := rng.Intn(10)

	switch pattern {
	case 0, 1, 2:
		return firstNames[rng.Intn(len(firstNames))]
	case 3, 4:
		return fmt.Sprintf("%s_%s",
			firstNames[rng.Intn(len(firstNames))],
			lastNames[rng.Intn(len(lastNames))])
	case 5, 6:
		return fmt.Sprintf("%s%d",
			firstNames[rng.Intn(len(firstNames))],
			rng.Intn(100))
	case 7:
		return fmt.Sprintf("%s_%s%d",
			firstNames[rng.Intn(len(firstNames))],
			lastNames[rng.Intn(len(lastNames))],
			rng.Intn(10))
	default:
		return fmt.Sprintf("user_%d", id)
	}
}

// GenerateRandomRating generates a random rating between min and max (inclusive)
func GenerateRandomRating(min, max int) int {
	return min + rng.Intn(max-min+1)
}

// GetRandomInt returns a random integer from 0 to n-1
func GetRandomInt(n int) int {
	return rng.Intn(n)
}
