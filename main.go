package main

import (
	"fmt"
	"github.com/gocolly/colly/v2"
	"github.com/manifoldco/promptui"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Subtitle struct {
	Author      string
	Description string
	Downloads   string
	Label       string
	Link        string
}

type TermSize struct {
	X int
	Y int
}

func handleError(e error) {
	if e != nil {
		fmt.Println("Error:", e.Error())
		os.Exit(1)
	}
}

func clearConsole() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func downloadFile(filePath string, fileUrl string, ext string) error {
	res, err := http.Get(fileUrl + ext)

	if res.StatusCode != 200 {
		return downloadFile(filePath, fileUrl, ".zip")
	}

	if err != nil {
		return err
	}

	defer res.Body.Close()

	out, err := os.Create(filePath + ext)

	if err != nil {
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, res.Body)
	fmt.Println("File downloaded")

	return err
}

func getTerminalSize() (*TermSize, error) {
	cmd := exec.Command("stty", "size")

	cmd.Stdin = os.Stdin

	out, err := cmd.Output()

	if err != nil {
		return nil, err
	}

	size := strings.Split(string(out), " ")
	y, err := strconv.Atoi(size[0])

	if err != nil {
		return nil, err
	}

	x, err := strconv.Atoi(strings.TrimSuffix(size[1], "\n"))

	if err != nil {
		return nil, err
	}

	termSize := &TermSize{
		X: x,
		Y: y,
	}

	return termSize, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: subs-finder 'finding dory'")
		os.Exit(1)
	}

	name := os.Args[1]
	baseUrl := fmt.Sprintf(
		"https://www.subdivx.com/index.php?accion=5&masdesc=&buscar=%s&oxdown=1",
		strings.Join(strings.Split(name, " "), "+"),
	)
	c := colly.NewCollector()
	renderOptions := true
	clearConsole()
	termSize, err := getTerminalSize()

	handleError(err)

	var subs []Subtitle

	c.OnHTML("#contenedor_izq", func(e *colly.HTMLElement) {
		var subtitle Subtitle
		var index = len(subs)

		e.ForEach("#menu_detalle_buscador", func(_ int, e *colly.HTMLElement) {
			subtitle.Link = e.ChildAttr(".titulo_menu_izq", "href")
			subs = append(subs, subtitle)
		})

		e.ForEach("#buscador_detalle", func(_ int, e *colly.HTMLElement) {
			subtitle.Author = e.ChildText(".link1")
			// long descriptions break the list render when moving through the options
			// https://github.com/manifoldco/promptui/issues/143
			subs[index].Description = e.ChildText("#buscador_detalle_sub")
			subs[index].Label = "Search term " + name
			subs[index].Downloads = strings.Split(e.ChildText("#buscador_detalle_sub_datos"), " ")[1]

			// the description is usually the longest field, this is to avoid the issue stated above
			// shortening the description so the row content doesn't break into a new line
			const X_OFFSET = 40
			var textLength = (len(subs[index].Author) +
				len(subs[index].Description) +
				len(subs[index].Downloads) + X_OFFSET)
			var lengthForDescription = (termSize.X -
				len(subs[index].Author) -
				len(subs[index].Downloads) -
				X_OFFSET)

			if textLength > termSize.X {
				subs[index].Description = subs[index].Description[:lengthForDescription] + "..."
			}
			index++
		})
	})

	c.OnScraped(func(r *colly.Response) {
		templates := &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "- {{ .Description }} By: {{ .Author | cyan }} (Downloads: {{ .Downloads | cyan }})",
			Inactive: "  {{ .Description }} By: {{ .Author }} (Downloads: {{ .Downloads }})",
			Selected: "{{ .Description | red }}",
		}

		label := fmt.Sprintf("Subtitles for '%s':", name)
		prompt := promptui.Select{
			Items:     subs,
			Size:      10,
			Label:     label,
			Templates: templates,
		}

		if renderOptions {
			i, _, err := prompt.Run()
			handleError(err)
			renderOptions = false
			c.Visit(subs[i].Link)
		}
	})

	c.OnHTML("#detalle_datos", func(e *colly.HTMLElement) {
		// download => https://www.subdivx.com/sub8/482353.rar
		u, err := url.Parse(e.ChildAttr(".link1", "href"))
		handleError(err)
		queryParams, _ := url.ParseQuery(u.RawQuery)
		id := queryParams["id"][0]
		subdir := queryParams["u"][0]

		if subdir == "1" {
			subdir = ""
		}
		downloadLink := fmt.Sprintf(
			"https://www.subdivx.com/sub%s/%s", subdir, id,
		)

		err = downloadFile("./"+id, downloadLink, ".rar")

		handleError(err)
	})

	c.OnError(func(r *colly.Response, e error) {
		handleError(e)
	})

	c.Visit(baseUrl)
}
