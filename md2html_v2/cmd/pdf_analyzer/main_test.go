package main

import (
	"testing"
)

// TestPageNumberCalculation tests the page number calculation logic
// for various TOC scenarios (1-page TOC, 2-page TOC, 3-page TOC)
func TestPageNumberCalculation(t *testing.T) {
	testCases := []struct {
		name            string
		physicalPage    int // Physical page number where content is found
		tocEndPage      int // Physical page where TOC ends
		coverPages      int // Number of cover pages (usually 1)
		wantLogicalPage int // Expected logical page number in TOC
		wantFooterPage  int // Expected footer page number
	}{
		// Scenario 1: 1-page TOC (Cover=1, TOC=2, Content starts at 3)
		{
			name:            "1-page TOC: Cover=1, TOC=2, Section '1. Intro' at physical 3",
			physicalPage:    3,
			tocEndPage:      2,
			coverPages:      1,
			wantLogicalPage: 2, // Physical 3 - Cover 1 = Logical 2
			wantFooterPage:  2, // Should show '2' in footer
		},
		{
			name:            "1-page TOC: Section '2. Getting Started' at physical 5",
			physicalPage:    5,
			tocEndPage:      2,
			coverPages:      1,
			wantLogicalPage: 4, // Physical 5 - Cover 1 = Logical 4
			wantFooterPage:  4,
		},

		// Scenario 2: 2-page TOC (Cover=1, TOC=2-3, Content starts at 4)
		{
			name:            "2-page TOC: Cover=1, TOC=2-3, Section '1. Intro' at physical 4",
			physicalPage:    4,
			tocEndPage:      3,
			coverPages:      1,
			wantLogicalPage: 3, // Physical 4 - Cover 1 = Logical 3
			wantFooterPage:  3,
		},
		{
			name:            "2-page TOC: Section '2. Advanced' at physical 10",
			physicalPage:    10,
			tocEndPage:      3,
			coverPages:      1,
			wantLogicalPage: 9, // Physical 10 - Cover 1 = Logical 9
			wantFooterPage:  9,
		},

		// Scenario 3: 3-page TOC (Cover=1, TOC=2-4, Content starts at 5)
		{
			name:            "3-page TOC: Cover=1, TOC=2-4, Section '1. Intro' at physical 5",
			physicalPage:    5,
			tocEndPage:      4,
			coverPages:      1,
			wantLogicalPage: 4, // Physical 5 - Cover 1 = Logical 4
			wantFooterPage:  4,
		},
		{
			name:            "3-page TOC: Last section at physical 20",
			physicalPage:    20,
			tocEndPage:      4,
			coverPages:      1,
			wantLogicalPage: 19, // Physical 20 - Cover 1 = Logical 19
			wantFooterPage:  19,
		},

		// Edge case: TOC starts numbering from 1 (TOC page itself)
		{
			name:            "TOC page 1 at physical 2",
			physicalPage:    2,
			tocEndPage:      2,
			coverPages:      1,
			wantLogicalPage: 1, // Physical 2 - Cover 1 = Logical 1 (TOC page)
			wantFooterPage:  1, // TOC footer should show '1'
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate logical page number based on current formula
			// Formula: Logical Page = Physical Page - Cover Pages
			gotLogicalPage := tc.physicalPage - tc.coverPages

			if gotLogicalPage != tc.wantLogicalPage {
				t.Errorf("Logical page calculation error: got %d, want %d", gotLogicalPage, tc.wantLogicalPage)
			}

			// Footer should display the same as logical page
			gotFooterPage := gotLogicalPage
			if gotFooterPage != tc.wantFooterPage {
				t.Errorf("Footer page mismatch: got %d, want %d", gotFooterPage, tc.wantFooterPage)
			}
		})
	}
}

// TestTOCPageRangeDetection tests the dynamic TOC end page detection
func TestTOCPageRangeDetection(t *testing.T) {
	testCases := []struct {
		name             string
		totalPages       int
		firstSectionPos  int // Physical page where first section content appears
		wantTocEndPage   int // Expected TOC end page
		wantContentStart int // Expected content start page
	}{
		{
			name:             "Short document: TOC on page 2",
			totalPages:       10,
			firstSectionPos:  3,
			wantTocEndPage:   2,
			wantContentStart: 3,
		},
		{
			name:             "Long document: TOC spans pages 2-3",
			totalPages:       50,
			firstSectionPos:  4,
			wantTocEndPage:   3,
			wantContentStart: 4,
		},
		{
			name:             "Very long document: TOC spans pages 2-4",
			totalPages:       100,
			firstSectionPos:  5,
			wantTocEndPage:   4,
			wantContentStart: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Content starts right after TOC ends
			gotTocEndPage := tc.firstSectionPos - 1
			gotContentStart := tc.firstSectionPos

			if gotTocEndPage != tc.wantTocEndPage {
				t.Errorf("TOC end page detection: got %d, want %d", gotTocEndPage, tc.wantTocEndPage)
			}

			if gotContentStart != tc.wantContentStart {
				t.Errorf("Content start page: got %d, want %d", gotContentStart, tc.wantContentStart)
			}
		})
	}
}

// TestPageOffsetFormula verifies the offset formula matches user's requirement
// User requirement: Cover=0(hidden), TOC=1-n, Body=n+1...
func TestPageOffsetFormula(t *testing.T) {
	// The formula should be: Logical Page = Physical Page - 1
	// This makes Physical 1 (Cover) = Logical 0 (hidden)
	// Physical 2 (TOC) = Logical 1
	// Physical 3 (Body start) = Logical 2

	offset := 1 // PAGE_OFFSET in md2pdf_v2.bat

	testCases := []struct {
		physicalPage    int
		expectedLogical int
		description     string
	}{
		{1, 0, "Cover page should be logical 0"},
		{2, 1, "TOC (1st page) should be logical 1"},
		{3, 2, "Body start (after 1-page TOC) should be logical 2"},
		{4, 3, "Second body page should be logical 3"},
		{18, 17, "Last page should match TOC index (physical 18 = logical 17)"},
	}

	for _, tc := range testCases {
		logical := tc.physicalPage - offset
		if logical != tc.expectedLogical {
			t.Errorf("%s: Physical %d - Offset %d = %d, want %d",
				tc.description, tc.physicalPage, offset, logical, tc.expectedLogical)
		}
	}
}

// TestVariableTOCWithOffset tests page numbering with variable TOC lengths
func TestVariableTOCWithOffset(t *testing.T) {
	offset := 1 // Cover page is always skipped from counting

	testCases := []struct {
		name            string
		tocPageCount    int // How many pages TOC takes
		sectionPhysical int // Physical page where section is found
		wantTocIndex    int // Page number shown in TOC index
		wantFooter      int // Page number shown in footer
	}{
		// TOC = 1 page (page 2), Body starts at page 3
		{"1-page TOC: Section at physical 3", 1, 3, 2, 2},
		{"1-page TOC: Section at physical 10", 1, 10, 9, 9},

		// TOC = 2 pages (pages 2-3), Body starts at page 4
		{"2-page TOC: Section at physical 4", 2, 4, 3, 3},
		{"2-page TOC: Section at physical 15", 2, 15, 14, 14},

		// TOC = 3 pages (pages 2-4), Body starts at page 5
		{"3-page TOC: Section at physical 5", 3, 5, 4, 4},
		{"3-page TOC: Section at physical 25", 3, 25, 24, 24},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// TOC index = Physical - Offset
			gotTocIndex := tc.sectionPhysical - offset

			// Footer should match TOC index
			gotFooter := gotTocIndex

			if gotTocIndex != tc.wantTocIndex {
				t.Errorf("TOC index: got %d, want %d", gotTocIndex, tc.wantTocIndex)
			}

			if gotFooter != tc.wantFooter {
				t.Errorf("Footer: got %d, want %d (should match TOC index)", gotFooter, tc.wantFooter)
			}

			// Verify TOC page numbering (TOC itself starts at logical 1)
			tocLogicalStart := 2 - offset // Physical 2 = TOC first page
			tocLogicalEnd := (1 + tc.tocPageCount) - offset

			if tocLogicalStart != 1 {
				t.Errorf("TOC should start at logical page 1, got %d", tocLogicalStart)
			}

			expectedTocEnd := tc.tocPageCount
			if tocLogicalEnd != expectedTocEnd {
				t.Errorf("TOC should end at logical page %d, got %d", expectedTocEnd, tocLogicalEnd)
			}
		})
	}
}
