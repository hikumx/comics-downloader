package sites

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/Girbons/comics-downloader/pkg/config"
	"github.com/Girbons/comics-downloader/pkg/core"
	"github.com/Girbons/comics-downloader/pkg/util"
	"github.com/anaskhan96/soup"
)

// Mangalivre representa o parser do Manga Livre.
type Mangalivre struct {
	options *config.Options
}

// NewMangalivre cria uma instância do parser.
func NewMangalivre(options *config.Options) *Mangalivre {
	return &Mangalivre{
		options: options,
	}
}

func (m *Mangalivre) isMangaURL(url string) bool {
	return strings.Contains(strings.ToLower(url), "mangalivre.blog/manga/")
}

func (m *Mangalivre) isChapterURL(url string) bool {
	return strings.Contains(strings.ToLower(url), "mangalivre.blog/capitulo/")
}

func (m *Mangalivre) getDocument(url string) (*soup.Root, error) {
	response, err := soup.Get(url)
	if err != nil {
		return nil, err
	}

	return soup.HTMLParse(response), nil
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func (m *Mangalivre) retrieveImageLinks(chapterURL string) ([]string, error) {
	doc, err := m.getDocument(chapterURL)
	if err != nil {
		return nil, err
	}

	var links []string

	// Estrutura observada no HTML do capítulo:
	// <div class="chapter-images">
	//   <img class="chapter-image" src="...">
	// </div>
	container := doc.Find("div", "class", "chapter-images")
	for _, image := range container.FindAll("img") {
		attrs := image.Attrs()

		imageURL := attrs["src"]
		if !util.IsURLValid(imageURL) {
			continue
		}

		links = append(links, imageURL)
	}

	// Fallback caso a estrutura mude
	if len(links) == 0 {
		for _, image := range doc.FindAll("img") {
			attrs := image.Attrs()

			className := attrs["class"]
			if !strings.Contains(className, "chapter-image") {
				continue
			}

			imageURL := attrs["src"]
			if util.IsURLValid(imageURL) {
				links = append(links, imageURL)
			}
		}
	}

	if len(links) == 0 {
		return nil, errors.New("nenhuma imagem encontrada no capítulo")
	}

	if m.options.Debug {
		m.options.Logger.Debug(
			fmt.Sprintf("Image Links found: %s", strings.Join(links, " ")),
		)
	}

	return links, nil
}

func (m *Mangalivre) retrieveIssueLinksFromManga(mangaURL string) ([]string, error) {
	doc, err := m.getDocument(mangaURL)
	if err != nil {
		return nil, err
	}

	var links []string
	seen := make(map[string]bool)

	// Estrutura observada na página base:
	// <li class="chapter-item">
	//   ...
	//   <a href="..." class="chapter-link">...</a>
	// </li>
	for _, item := range doc.FindAll("li", "class", "chapter-item") {
		linkElem := item.Find("a", "class", "chapter-link")
		if linkElem.Pointer == nil {
			continue
		}

		chapterURL := linkElem.Attrs()["href"]
		if !m.isChapterURL(chapterURL) || !util.IsURLValid(chapterURL) {
			continue
		}

		if !seen[chapterURL] {
			links = append(links, chapterURL)
			seen[chapterURL] = true
		}
	}

	// Fallback: procura qualquer link de capítulo no documento
	if len(links) == 0 {
		for _, element := range doc.FindAll("a") {
			chapterURL := element.Attrs()["href"]

			if !m.isChapterURL(chapterURL) || !util.IsURLValid(chapterURL) {
				continue
			}

			if !seen[chapterURL] {
				links = append(links, chapterURL)
				seen[chapterURL] = true
			}
		}
	}

	if len(links) == 0 {
		return nil, errors.New("nenhum capítulo encontrado")
	}

	if m.options.Debug {
		m.options.Logger.Debug(
			fmt.Sprintf("Issues Links found: %s", strings.Join(links, " ")),
		)
	}

	return links, nil
}

// RetrieveIssueLinks retorna um capítulo ou todos os capítulos do mangá.
func (m *Mangalivre) RetrieveIssueLinks() ([]string, error) {
	url := strings.TrimSpace(m.options.URL)

	if m.isChapterURL(url) {
		return []string{url}, nil
	}

	if !m.isMangaURL(url) {
		return nil, errors.New("URL não suportada")
	}

	links, err := m.retrieveIssueLinksFromManga(url)
	if err != nil {
		return nil, err
	}

	// Sem -all, baixa apenas o capítulo mais recente (último da lista)
	if !m.options.All {
		return []string{links[len(links)-1]}, nil
	}

	return links, nil
}

func (m *Mangalivre) extractMangaName(url string) string {
	parts := util.TrimAndSplitURL(url)

	for i, part := range parts {
		if part == "manga" && i+1 < len(parts) {
			return strings.ReplaceAll(parts[i+1], "-", " ")
		}
	}

	return ""
}

func (m *Mangalivre) extractChapterNumber(url string) string {
	parts := util.TrimAndSplitURL(url)

	if len(parts) == 0 {
		return ""
	}

	last := parts[len(parts)-1]
	last = strings.TrimSuffix(last, "/")

	// Exemplo:
	// super-no-ura-de-yani-suu-futari-capitulo-16-5-extra-volume-1
	re := regexp.MustCompile(`-capitulo-([0-9]+(?:-[0-9]+)?)`)
	match := re.FindStringSubmatch(last)

	if len(match) >= 2 {
		return strings.ReplaceAll(match[1], "-", ".")
	}

	return ""
}

// GetInfo extrai nome do mangá e número do capítulo.
func (m *Mangalivre) GetInfo(url string) (string, string) {
	doc, err := m.getDocument(url)
	if err == nil {
		title := doc.Find("h1", "class", "manga-title").Text()
		title = cleanText(title)

		if strings.Contains(title, "/") {
			title = strings.Split(title, "/")[0]
		}

		title = cleanText(title)
		if title != "" {
			return title, m.extractChapterNumber(url)
		}
	}

	return m.extractMangaName(url), m.extractChapterNumber(url)
}

// Initialize baixa os links das páginas do capítulo.
func (m *Mangalivre) Initialize(comic *core.Comic) error {
	if !m.isChapterURL(comic.URLSource) {
		return errors.New("Initialize requer uma URL de capítulo")
	}

	name, issueNumber := m.GetInfo(comic.URLSource)

	comic.Name = name
	comic.IssueNumber = issueNumber

	links, err := m.retrieveImageLinks(comic.URLSource)
	if err != nil {
		return err
	}

	comic.Links = links

	return nil
}
