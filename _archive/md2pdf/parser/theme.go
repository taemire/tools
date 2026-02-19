// Package parser provides functionality for parsing theme JSON and markdown files
package parser

import (
	"encoding/json"
	"os"
)

// Theme represents the complete theme configuration
type Theme struct {
	Name        string        `json:"name"`
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Page        PageConfig    `json:"page"`
	Fonts       FontsConfig   `json:"fonts"`
	Colors      ColorsConfig  `json:"colors"`
	Cover       CoverConfig   `json:"cover"`
	TOC         TOCConfig     `json:"toc"`
	Content     ContentConfig `json:"content"`
}

// PageConfig defines page size and margins
type PageConfig struct {
	Size        string        `json:"size"`
	Orientation string        `json:"orientation"`
	Margins     MarginsConfig `json:"margins"`
}

type MarginsConfig struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

// FontsConfig defines font families
type FontsConfig struct {
	Primary FontFamily `json:"primary"`
	Code    FontFamily `json:"code"`
}

type FontFamily struct {
	Family  string `json:"family"`
	Regular string `json:"regular"`
	Bold    string `json:"bold"`
}

// ColorsConfig defines color palette
type ColorsConfig struct {
	Primary    string           `json:"primary"`
	Secondary  string           `json:"secondary"`
	Accent     string           `json:"accent"`
	Text       TextColors       `json:"text"`
	Background BackgroundColors `json:"background"`
	Border     string           `json:"border"`
}

type TextColors struct {
	Default string `json:"default"`
	Muted   string `json:"muted"`
	Light   string `json:"light"`
}

type BackgroundColors struct {
	Default string `json:"default"`
	Alt     string `json:"alt"`
	Code    string `json:"code"`
}

// CoverConfig defines cover page layout
type CoverConfig struct {
	Enabled   bool            `json:"enabled"`
	TopBorder TopBorderConfig `json:"topBorder"`
	Logo      LogoConfig      `json:"logo"`
	Title     TitleConfig     `json:"title"`
	Subtitle  SubtitleConfig  `json:"subtitle"`
	InfoTable InfoTableConfig `json:"infoTable"`
	Copyright CopyrightConfig `json:"copyright"`
}

type TopBorderConfig struct {
	Enabled bool    `json:"enabled"`
	Height  float64 `json:"height"`
	Color   string  `json:"color"`
}

type LogoConfig struct {
	Position PositionConfig `json:"position"`
	FontSize float64        `json:"fontSize"`
	Color    string         `json:"color"`
}

type PositionConfig struct {
	X interface{} `json:"x"` // can be float64 or string like "right"
	Y float64     `json:"y"`
}

type TitleConfig struct {
	Position   PositionConfig `json:"position"`
	FontSize   float64        `json:"fontSize"`
	FontWeight string         `json:"fontWeight"`
	Color      string         `json:"color"`
}

type SubtitleConfig struct {
	Position   PositionConfig   `json:"position"`
	FontSize   float64          `json:"fontSize"`
	Color      string           `json:"color"`
	LeftBorder LeftBorderConfig `json:"leftBorder"`
}

type LeftBorderConfig struct {
	Enabled bool    `json:"enabled"`
	Width   float64 `json:"width"`
	Color   string  `json:"color"`
}

type InfoTableConfig struct {
	Position        PositionConfig `json:"position"`
	LabelWidth      float64        `json:"labelWidth"`
	ValueWidth      float64        `json:"valueWidth"`
	RowHeight       float64        `json:"rowHeight"`
	LabelBackground string         `json:"labelBackground"`
	FontSize        float64        `json:"fontSize"`
	Fields          []string       `json:"fields"`
}

type CopyrightConfig struct {
	Position PositionConfig `json:"position"`
	FontSize float64        `json:"fontSize"`
	Color    string         `json:"color"`
	Template string         `json:"template"`
}

// TOCConfig defines table of contents layout
type TOCConfig struct {
	Enabled    bool                      `json:"enabled"`
	Title      TOCTitleConfig            `json:"title"`
	Background TOCBgConfig               `json:"background"`
	Item       TOCItemConfig             `json:"item"`
	Levels     map[string]TOCLevelConfig `json:"levels"`
	Header     struct{ Enabled bool }    `json:"header"`
	Footer     struct{ Enabled bool }    `json:"footer"`
}

type TOCTitleConfig struct {
	Text     string  `json:"text"`
	FontSize float64 `json:"fontSize"`
	Color    string  `json:"color"`
}

type TOCBgConfig struct {
	Enabled      bool    `json:"enabled"`
	Color        string  `json:"color"`
	Padding      float64 `json:"padding"`
	BorderRadius float64 `json:"borderRadius"`
}

type TOCItemConfig struct {
	FontSize        float64         `json:"fontSize"`
	Color           string          `json:"color"`
	LineHeight      float64         `json:"lineHeight"`
	ShowPageNumber  bool            `json:"showPageNumber"`
	PageNumberAlign string          `json:"pageNumberAlign"`
	DotLeader       DotLeaderConfig `json:"dotLeader"`
	Clickable       bool            `json:"clickable"`
}

type DotLeaderConfig struct {
	Enabled bool    `json:"enabled"`
	Char    string  `json:"char"`
	Spacing float64 `json:"spacing"`
	Color   string  `json:"color"`
}

type TOCLevelConfig struct {
	Indent   float64 `json:"indent"`
	FontSize float64 `json:"fontSize"`
	Bold     bool    `json:"bold"`
}

// ContentConfig defines content area styling
type ContentConfig struct {
	Header     HeaderConfig     `json:"header"`
	Footer     FooterConfig     `json:"footer"`
	Heading    HeadingConfig    `json:"heading"`
	Paragraph  ParagraphConfig  `json:"paragraph"`
	Code       CodeConfig       `json:"code"`
	Table      TableConfig      `json:"table"`
	List       ListConfig       `json:"list"`
	Blockquote BlockquoteConfig `json:"blockquote"`
	Links      LinksConfig      `json:"links"`
}

type HeaderConfig struct {
	Enabled  bool               `json:"enabled"`
	Height   float64            `json:"height"`
	Text     string             `json:"text"`
	FontSize float64            `json:"fontSize"`
	Color    string             `json:"color"`
	Align    string             `json:"align"`
	Border   BorderBottomConfig `json:"border"`
}

type BorderBottomConfig struct {
	Bottom bool   `json:"bottom"`
	Color  string `json:"color"`
}

type FooterConfig struct {
	Enabled bool             `json:"enabled"`
	Height  float64          `json:"height"`
	Left    FooterTextConfig `json:"left"`
	Right   FooterTextConfig `json:"right"`
	Border  BorderTopConfig  `json:"border"`
}

type FooterTextConfig struct {
	Text     string  `json:"text"`
	FontSize float64 `json:"fontSize"`
	Color    string  `json:"color"`
}

type BorderTopConfig struct {
	Top   bool   `json:"top"`
	Color string `json:"color"`
}

type HeadingConfig struct {
	H1 HeadingStyleConfig `json:"h1"`
	H2 HeadingStyleConfig `json:"h2"`
	H3 HeadingStyleConfig `json:"h3"`
	H4 HeadingStyleConfig `json:"h4"`
}

type HeadingStyleConfig struct {
	FontSize        float64 `json:"fontSize"`
	Color           string  `json:"color"`
	MarginTop       float64 `json:"marginTop"`
	MarginBottom    float64 `json:"marginBottom"`
	Underline       bool    `json:"underline,omitempty"`
	UnderlineColor  string  `json:"underlineColor,omitempty"`
	LeftBorder      bool    `json:"leftBorder,omitempty"`
	LeftBorderWidth float64 `json:"leftBorderWidth,omitempty"`
	LeftBorderColor string  `json:"leftBorderColor,omitempty"`
}

type ParagraphConfig struct {
	FontSize   float64 `json:"fontSize"`
	LineHeight float64 `json:"lineHeight"`
	Color      string  `json:"color"`
}

type CodeConfig struct {
	Inline InlineCodeConfig `json:"inline"`
	Block  BlockCodeConfig  `json:"block"`
}

type InlineCodeConfig struct {
	FontSize        float64   `json:"fontSize"`
	FontFamily      string    `json:"fontFamily"`
	BackgroundColor string    `json:"backgroundColor"`
	Padding         PaddingHV `json:"padding"`
	BorderRadius    float64   `json:"borderRadius"`
	Color           string    `json:"color"`
}

type BlockCodeConfig struct {
	FontSize        float64 `json:"fontSize"`
	FontFamily      string  `json:"fontFamily"`
	BackgroundColor string  `json:"backgroundColor"`
	Color           string  `json:"color"`
	Padding         float64 `json:"padding"`
	BorderRadius    float64 `json:"borderRadius"`
	LineNumbers     bool    `json:"lineNumbers"`
}

type PaddingHV struct {
	Horizontal float64 `json:"horizontal"`
	Vertical   float64 `json:"vertical"`
}

type TableConfig struct {
	HeaderBackground string    `json:"headerBackground"`
	HeaderColor      string    `json:"headerColor"`
	BorderColor      string    `json:"borderColor"`
	CellPadding      PaddingHV `json:"cellPadding"`
	AltRowBackground string    `json:"altRowBackground"`
}

type ListConfig struct {
	Bullet   ListStyleConfig `json:"bullet"`
	Numbered ListStyleConfig `json:"numbered"`
}

type ListStyleConfig struct {
	FontSize float64 `json:"fontSize"`
	Indent   float64 `json:"indent"`
	Spacing  float64 `json:"spacing"`
}

type BlockquoteConfig struct {
	BackgroundColor string           `json:"backgroundColor"`
	LeftBorder      LeftBorderConfig `json:"leftBorder"`
	Padding         PaddingHV        `json:"padding"`
	BorderRadius    float64          `json:"borderRadius"`
}

type LinksConfig struct {
	Internal LinkStyleConfig `json:"internal"`
	External LinkStyleConfig `json:"external"`
}

type LinkStyleConfig struct {
	Color     string `json:"color"`
	Underline bool   `json:"underline"`
	Clickable bool   `json:"clickable"`
}

// LoadTheme loads a theme from a JSON file
func LoadTheme(path string) (*Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return nil, err
	}

	return &theme, nil
}

// GetDefaultTheme returns the default corporate-blue theme path
func GetDefaultThemePath() string {
	return "themes/corporate-blue.json"
}
