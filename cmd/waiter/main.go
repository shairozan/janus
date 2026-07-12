package main

import (
	"crypto/rand"
	"log"
	"math/big"
	"os"
	"time"
)

func main() {
	words := []string{
		"apple", "banana", "cherry", "dragon", "elephant", "forest", "guitar", "harmony",
		"island", "jungle", "kitchen", "library", "mountain", "notebook", "ocean", "piano",
		"quartz", "rainbow", "sunshine", "telescope", "umbrella", "volcano", "whisper", "xylophone",
		"yellow", "zebra", "adventure", "butterfly", "cascade", "diamond", "emerald", "firefly",
		"galaxy", "horizon", "infinity", "journey", "kaleidoscope", "lantern", "melody", "nebula",
		"oasis", "paradise", "quantum", "rhythm", "serenity", "thunder", "universe", "velocity",
		"wonder", "xenial", "yearning", "zephyr",
	}

	// Print a random word every second for 30 seconds
	for i := 0; i < 30; i++ {
		// Use crypto/rand for secure random number generation
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
		if err != nil {
			log.Printf("Error generating random number: %v", err)

			continue
		}
		randomWord := words[n.Int64()]
		log.Print(randomWord)
		time.Sleep(1 * time.Second)
	}

	os.Exit(0)
}
