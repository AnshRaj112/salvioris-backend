package backend

import (
	"fmt"
	"math"
	"strings"
)

func GetBanner() string {
	lines := []string{
		` 
 ________  ________  ___       ___      ___ ___  ________  ________  ___  ________      
|\   ____\|\   __  \|\  \     |\  \    /  /|\  \|\   __  \|\   __  \|\  \|\   ____\     
\ \  \___|\ \  \|\  \ \  \    \ \  \  /  / | \  \ \  \|\  \ \  \|\  \ \  \ \  \___|_    
 \ \_____  \ \   __  \ \  \    \ \  \/  / / \ \  \ \  \\\  \ \   _  _\ \  \ \_____  \   
  \|____|\  \ \  \ \  \ \  \____\ \    / /   \ \  \ \  \\\  \ \  \\  \\ \  \|____|\  \  
    ____\_\  \ \__\ \__\ \_______\ \__/ /     \ \__\ \_______\ \__\\ _\\ \__\____\_\  \ 
   |\_________\|__|\|__|\|_______|\|__|/       \|__|\|_______|\|__|\|__|\|__|\_________\
   \|_________|                                                             \|_________|
                                                                                        
                                                                                        `,
	}

	var output strings.Builder
	output.WriteString("\n")

	// Sunset Gradient: Orange (#f97316) → Rose (#e11d48)
	r1, g1, b1 := 249.0, 115.0, 22.0
	r2, g2, b2 := 225.0, 29.0, 72.0

	allLines := strings.Split(lines[0], "\n")
	for _, line := range allLines {
		for x, char := range line {
			if char == ' ' {
				output.WriteRune(char)
				continue
			}

			ratio := float64(x) / 95.0
			if ratio > 1.0 {
				ratio = 1.0
			}

			r := int(math.Round(r1 + (r2-r1)*ratio))
			g := int(math.Round(g1 + (g2-g1)*ratio))
			b := int(math.Round(b1 + (b2-b1)*ratio))

			output.WriteString(
				fmt.Sprintf("\033[38;2;%d;%d;%dm%c\033[0m", r, g, b, char),
			)
		}
		output.WriteString("\n")
	}

	return output.String()
}