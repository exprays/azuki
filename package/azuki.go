package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kkdai/youtube/v2"
	"github.com/manifoldco/promptui"
)

const azukiArt = `
    _    _____  _   _ _  _____ 
   / \  |__  / | | | | |/ /_ _|
  / _ \   / /  | | | | ' / | | 
 / ___ \ / /_  | |_| | . \ | | 
/_/   \_/____|  \___/|_|\_\___|
`

const builtByText = `
Built with ❤️  by surya
`

// ProgressBar represents a custom progress bar
type ProgressBar struct {
	total      int64
	current    int64
	width      int
	lastUpdate time.Time
	speed      float64 // bytes per second
}

func NewProgressBar(total int64) *ProgressBar {
	return &ProgressBar{
		total:      total,
		width:      50, // Width of the progress bar
		lastUpdate: time.Now(),
	}
}

func (pb *ProgressBar) Update(n int) {
	pb.current += int64(n)
	now := time.Now()
	elapsed := now.Sub(pb.lastUpdate).Seconds()

	// Update speed every 0.5 seconds
	if elapsed >= 0.5 {
		pb.speed = float64(n) / elapsed
		pb.lastUpdate = now
	}

	percentage := float64(pb.current) * 100 / float64(pb.total)
	completed := int(float64(pb.width) * float64(pb.current) / float64(pb.total))

	// Clear the current line
	fmt.Printf("\r\033[K")

	// Print progress bar
	color.Set(color.FgCyan)
	fmt.Print("[")

	// Print completed portion
	color.Set(color.FgGreen)
	for i := 0; i < completed; i++ {
		fmt.Print("█")
	}

	// Print remaining portion
	color.Set(color.FgWhite)
	for i := completed; i < pb.width; i++ {
		fmt.Print("░")
	}

	color.Set(color.FgCyan)
	fmt.Print("]")

	// Print percentage and speed
	color.Set(color.FgYellow)
	fmt.Printf(" %.1f%% ", percentage)

	// Print transfer speed
	color.Set(color.FgMagenta)
	if pb.speed > 1024*1024 {
		fmt.Printf("(%.2f MB/s)", pb.speed/1024/1024)
	} else if pb.speed > 1024 {
		fmt.Printf("(%.2f KB/s)", pb.speed/1024)
	} else {
		fmt.Printf("(%.0f B/s)", pb.speed)
	}

	// Print downloaded size / total size
	color.Set(color.FgBlue)
	fmt.Printf(" [%s/%s]",
		formatSize(pb.current),
		formatSize(pb.total))

	color.Unset()
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func main() {
	// Print ASCII art with color
	color.Cyan(azukiArt)
	color.Yellow(builtByText)

	// Platform selection
	prompt := promptui.Select{
		Label: "Select Platform",
		Items: []string{"YouTube", "Instagram"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . | cyan }}",
			Active:   "→ {{ . | red }}",
			Inactive: "  {{ . | white }}",
			Selected: "✔ {{ . | green }}",
		},
	}

	_, platform, err := prompt.Run()
	if err != nil {
		fmt.Printf("Platform selection failed: %v\n", err)
		return
	}

	switch platform {
	case "YouTube":
		handleYouTube()
	case "Instagram":
		handleInstagram()
	}
}

func handleYouTube() {
	reader := bufio.NewReader(os.Stdin)

	color.Blue("Enter YouTube URL: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	client := youtube.Client{}
	video, err := client.GetVideo(url)
	if err != nil {
		color.Red("Failed to get video: %v\n", err)
		return
	}

	color.Green("\nVideo Title: %s\n", video.Title)

	var formats []youtube.Format
	var formatStrings []string
	for _, format := range video.Formats {
		if format.Quality != "" {
			formats = append(formats, format)
			formatString := fmt.Sprintf("Quality: %s | Format: %s",
				format.Quality,
				format.MimeType)
			formatStrings = append(formatStrings, formatString)
		}
	}

	prompt := promptui.Select{
		Label: "Select Quality",
		Items: formatStrings,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . | cyan }}",
			Active:   "→ {{ . | yellow }}",
			Inactive: "  {{ . | white }}",
			Selected: "✔ {{ . | green }}",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		color.Red("Quality selection failed: %v\n", err)
		return
	}

	selectedFormat := formats[idx]

	if err := os.MkdirAll("downloads", 0755); err != nil {
		color.Red("Failed to create downloads directory: %v\n", err)
		return
	}

	filename := sanitizeFilename(video.Title)
	ext := strings.Split(selectedFormat.MimeType, "/")[1]
	if i := strings.Index(ext, ";"); i != -1 {
		ext = ext[:i]
	}
	filepath := filepath.Join("downloads", filename+"."+ext)

	color.Yellow("\nInitiating download...\n")

	resp, _, err := client.GetStream(video, &selectedFormat)
	if err != nil {
		color.Red("Failed to get stream: %v\n", err)
		return
	}
	defer resp.Close()

	file, err := os.Create(filepath)
	if err != nil {
		color.Red("Failed to create file: %v\n", err)
		return
	}
	defer file.Close()

	progressBar := NewProgressBar(selectedFormat.ContentLength)
	buffer := make([]byte, 1024*1024) // 1MB buffer

	for {
		n, err := resp.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			color.Red("\nError reading stream: %v\n", err)
			return
		}

		if _, err := file.Write(buffer[:n]); err != nil {
			color.Red("\nError writing to file: %v\n", err)
			return
		}

		progressBar.Update(n)
	}

	fmt.Println() // New line after progress bar
	color.Green("\n✔ Download completed: %s\n", filepath)
}

func handleInstagram() {
	prompt := promptui.Select{
		Label: "Select Download Type",
		Items: []string{"Download Photo", "Download Video"},
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . | cyan }}",
			Active:   "→ {{ . | red }}",
			Inactive: "  {{ . | white }}",
			Selected: "✔ {{ . | green }}",
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		color.Red("Selection failed: %v\n", err)
		return
	}

	color.Yellow("Instagram %s functionality coming soon!", result)
}

func sanitizeFilename(filename string) string {
	invalidChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	for _, char := range invalidChars {
		filename = strings.ReplaceAll(filename, char, "")
	}
	filename = strings.TrimSpace(filename)
	filename = strings.ReplaceAll(filename, " ", "_")
	return filename
}
