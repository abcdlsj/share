package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GoogleFontsResponse struct {
	Kind  string `json:"kind"`
	Items []struct {
		Family       string   `json:"family"`
		Variants     []string `json:"variants"`
		Subsets      []string `json:"subsets"`
		Version      string   `json:"version"`
		LastModified string   `json:"lastModified"`
		Files        struct {
			Regular string `json:"regular"`
			Italic  string `json:"italic"`
		} `json:"files"`
		Category string `json:"category"`
		Kind     string `json:"kind"`
		Menu     string `json:"menu"`
	} `json:"items"`
}

func main() {
	r := gin.Default()

	r.SetHTMLTemplate(template.Must(template.New("font_preview.html").Funcs(template.FuncMap{}).Parse(fontPreviewTemplate)))

	// Define a route to handle the font preview page
	r.GET("/", handleFontPreview)

	// Start the server
	r.Run(":8080")
}

func handleFontPreview(c *gin.Context) {
	// Fetch the available fonts from Google Fonts API
	response, err := http.Get("https://www.googleapis.com/webfonts/v1/webfonts?key=AIzaSyARTiYTEmhDVi3wPvPhuoSOyEWysZ6_CWs")
	if err != nil {
		fmt.Printf("Error fetching fonts: %v", err)
		return
	}
	defer response.Body.Close()

	// Parse the JSON response into FontPreviewData struct
	var data GoogleFontsResponse
	err = json.NewDecoder(response.Body).Decode(&data)
	if err != nil {
		fmt.Printf("Error parsing fonts: %v", err)
		return
	}
	defer response.Body.Close()

	fmt.Printf("Selected font: %+v\n", data)

	// Render the font preview template with the fetched data
	c.HTML(http.StatusOK, "font_preview.html", gin.H{
		"Fonts": data,
	})
}

// HTML template for font preview
var fontPreviewTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Font Preview</title>
</head>
<body>
    <h1>Font Preview</h1>
    <ul>
        {{range .Fonts.Items}}
        <li style="font-family: '{{.Family}}', sans-serif;">{{.Family}}</li>
        {{end}}
    </ul>
</body>
</html>
`
