package service

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
	"github.com/stemsi/exstem-backend/internal/model"
)

// SchoolInfo holds school branding data fetched from app_settings.
type SchoolInfo struct {
	Name    string // e.g. "SMA Negeri 1"
	LogoURL string // e.g. "/uploads/uuid.png" (relative URL stored in DB)
}

// ---------------------------------------------------------------------------
// Layout constants — all dimensions in millimeters unless noted otherwise.
// Adjust these values to fine-tune the card appearance without touching logic.
// ---------------------------------------------------------------------------

const (
	// Page
	pdfPageWidthMM  = 210.0 // A4 width
	pdfPageHeightMM = 297.0 // A4 height
	pdfPageMarginMM = 12.0  // margin on all four sides

	// Grid
	pdfCols     = 3   // columns per row
	pdfGutterMM = 0.0 // horizontal gap between columns
	pdfRowGapMM = 0.0 // vertical gap between rows

	// Card chrome
	pdfCardPadMM = 3.5  // inner padding (top of body / bottom of card)
	pdfHeaderHMM = 12.5 // height of the coloured header bar

	// Body field sizing
	pdfLabelHMM   = 4.0 // vertical space reserved for a small-caps label
	pdfValueHMM   = 4.5 // vertical space reserved for the bold value below it
	pdfLogoSizeMM = 7.5 // logo square side length inside the header

	// Vertical gaps between body sections
	pdfSectionGapMM    = 1.0  // gap after Name row → before Username/Class row
	pdfBeforeDashGapMM = -0.5 // gap after Username/Class → dashed separator
	pdfDashSepGapMM    = 4.0  // gap after dashed separator → Password row
)

// Font identifiers registered with gopdf.
const (
	fontRegular = "roboto"
	fontBold    = "roboto-bold"
)

// Font directory relative to the working directory (project root).
const pdfFontsDir = "internal/assets/fonts"

// Header text styling (font sizes in PDF points).
const (
	schoolNameFontPt = 5.5
	titleFontPt      = 6.5
	headerTextGapMM  = 0.8 // vertical gap between school name and title
)

// ---------------------------------------------------------------------------
// Derived helpers
// ---------------------------------------------------------------------------

// mmToPt converts millimeters to PDF points (1 mm ≈ 2.83465 pt).
func mmToPt(mm float64) float64 { return mm * 2.83465 }

// pdfCardHeightMM computes the total card height from the layout constants.
func pdfCardHeightMM() float64 {
	return pdfHeaderHMM +
		(pdfCardPadMM*2 - 2.0) + // top + (trimmed) bottom padding
		pdfLabelHMM + pdfValueHMM + // Name
		pdfSectionGapMM +
		pdfLabelHMM + pdfValueHMM + // Username & Class
		pdfBeforeDashGapMM +
		pdfDashSepGapMM +
		pdfLabelHMM + pdfValueHMM // Password
}

// pdfMaxRowsPerPage calculates how many card rows fit on one A4 page.
func pdfMaxRowsPerPage() int {
	usable := pdfPageHeightMM - 2*pdfPageMarginMM
	cardH := pdfCardHeightMM()
	rows := 0
	for y := 0.0; y+cardH <= usable; y += cardH + pdfRowGapMM {
		rows++
	}
	return rows
}

// ---------------------------------------------------------------------------
// Logo helpers
// ---------------------------------------------------------------------------

// resolveLogoPath converts a stored URL path (e.g. "/uploads/uuid.png") into
// a local filesystem path. Returns "" if the file does not exist.
func resolveLogoPath(logoURL string) string {
	if logoURL == "" {
		return ""
	}
	localPath := strings.TrimPrefix(logoURL, "/")
	if _, err := os.Stat(localPath); err != nil {
		return ""
	}
	return localPath
}

// loadLogoAsJPEG reads an image file and re-encodes it as an 8-bit JPEG in
// memory.  This guarantees compatibility with gopdf, which does not support
// 16-bit PNGs, interlaced PNGs, or certain colour profiles.
func loadLogoAsJPEG(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open logo: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode logo: %w", err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("re-encode logo as JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// GenerateStudentCardsPDF builds an A4 PDF containing student ID cards
// arranged in a 3-column grid.  Returns the raw PDF bytes.
func GenerateStudentCardsPDF(cards []model.StudentCardInfo, school SchoolInfo) ([]byte, error) {
	if len(cards) == 0 {
		return nil, fmt.Errorf("no student cards to generate")
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})

	// Register fonts.
	for name, file := range map[string]string{
		fontRegular: "Roboto-Regular.ttf",
		fontBold:    "Roboto-Bold.ttf",
	} {
		if err := pdf.AddTTFFont(name, filepath.Join(pdfFontsDir, file)); err != nil {
			return nil, fmt.Errorf("load font %s: %w", name, err)
		}
	}

	// Pre-compute grid dimensions.
	rowsPerPage := pdfMaxRowsPerPage()
	cardsPerPage := pdfCols * rowsPerPage
	usableW := pdfPageWidthMM - 2*pdfPageMarginMM
	cardW := (usableW - float64(pdfCols-1)*pdfGutterMM) / float64(pdfCols)
	cardH := pdfCardHeightMM()

	// Load the school logo once (shared across all cards).
	logoBytes, _ := loadLogoAsJPEG(resolveLogoPath(school.LogoURL))

	for i, card := range cards {
		if i%cardsPerPage == 0 {
			pdf.AddPage()
		}

		pos := i % cardsPerPage
		col := pos % pdfCols
		row := pos / pdfCols

		x := pdfPageMarginMM + float64(col)*(cardW+pdfGutterMM)
		y := pdfPageMarginMM + float64(row)*(cardH+pdfRowGapMM)

		if err := drawStudentCard(pdf, card, school.Name, logoBytes, x, y, cardW, cardH); err != nil {
			return nil, fmt.Errorf("draw card (student_id=%d): %w", card.ID, err)
		}
	}

	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	pdf.Close()

	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Card drawing
// ---------------------------------------------------------------------------

// drawStudentCard renders a single student ID card at (xMM, yMM).
func drawStudentCard(pdf *gopdf.GoPdf, card model.StudentCardInfo, schoolName string, logoBytes []byte, xMM, yMM, wMM, hMM float64) error {
	x, y := mmToPt(xMM), mmToPt(yMM)
	w, h := mmToPt(wMM), mmToPt(hMM)
	pad := mmToPt(pdfCardPadMM)

	// ── Card border ──────────────────────────────────────────────────────
	pdf.SetStrokeColor(200, 200, 200)
	pdf.SetLineWidth(0.5)
	pdf.RectFromUpperLeftWithStyle(x, y, w, h, "D")

	// ── Header bar (filled + stroked) ────────────────────────────────────
	headerH := mmToPt(pdfHeaderHMM)
	pdf.SetFillColor(245, 247, 250)
	pdf.SetStrokeColor(200, 205, 215)
	pdf.SetLineWidth(0.5)
	pdf.RectFromUpperLeftWithStyle(x, y, w, headerH, "FD")

	drawHeaderContent(pdf, x, y, w, headerH, schoolName, logoBytes)

	// ── Body ─────────────────────────────────────────────────────────────
	curY := y + headerH + pad
	contentW := w - 2*pad

	// Row 1 — Student name
	curY = drawFieldRow(pdf, x+pad, curY, contentW, "NAMA SISWA", card.Name, 8.5)
	curY += mmToPt(pdfSectionGapMM)

	// Row 2 — Username (NISN) & Class (side by side)
	halfW := (contentW - mmToPt(pdfGutterMM)) / 2
	drawFieldRow(pdf, x+pad, curY, halfW, "USERNAME (NISN)", card.NISN, 9)
	drawFieldRow(pdf, x+pad+halfW+mmToPt(pdfGutterMM), curY, halfW, "KELAS", card.ClassName, 8)
	curY += mmToPt(pdfLabelHMM + pdfValueHMM)
	curY += mmToPt(pdfBeforeDashGapMM)

	// Dashed separator
	pdf.SetStrokeColor(200, 210, 220)
	pdf.SetLineWidth(0.3)
	pdf.SetLineType("dashed")
	pdf.Line(x+pad, curY, x+w-pad, curY)
	pdf.SetLineType("")
	curY += mmToPt(pdfDashSepGapMM)

	// Row 3 — Password
	drawFieldLabel(pdf, x+pad, curY, "PASSWORD UJIAN")
	curY += mmToPt(pdfLabelHMM)

	password := card.Password
	if password == "" {
		password = "-"
	}
	if err := pdf.SetFont(fontBold, "", 11); err != nil {
		return err
	}
	pdf.SetTextColor(20, 30, 40)
	pdf.SetXY(x+pad, curY)
	pdf.Text(password)

	return nil
}

// ---------------------------------------------------------------------------
// Header content (logo + school name + title)
// ---------------------------------------------------------------------------

// drawHeaderContent renders the centred logo, school name, and card title
// inside the header bar.
func drawHeaderContent(pdf *gopdf.GoPdf, x, y, w, headerH float64, schoolName string, logoBytes []byte) {
	schoolName = strings.ToUpper(schoolName)
	const title = "KARTU PESERTA UJIAN"

	// Measure text widths.
	_ = pdf.SetFont(fontBold, "", schoolNameFontPt)
	schoolNameW, _ := pdf.MeasureTextWidth(schoolName)
	_ = pdf.SetFont(fontBold, "", titleFontPt)
	titleW, _ := pdf.MeasureTextWidth(title)

	maxTextW := schoolNameW
	if titleW > schoolNameW {
		maxTextW = titleW
	}

	// Compute horizontal centre for the [logo | text] block.
	logoSize := mmToPt(pdfLogoSizeMM)
	logoGap := mmToPt(1.5)

	totalContentW := maxTextW
	hasLogo := len(logoBytes) > 0
	if hasLogo {
		totalContentW += logoSize + logoGap
	}
	startX := x + (w-totalContentW)/2

	// ── Logo ─────────────────────────────────────────────────────────────
	curX := startX
	if hasLogo {
		logoY := y + (headerH-logoSize)/2
		if holder, err := gopdf.ImageHolderByBytes(logoBytes); err == nil {
			if err := pdf.ImageByHolder(holder, curX, logoY, &gopdf.Rect{W: logoSize, H: logoSize}); err != nil {
				// Silently skip logo on draw error — the card is still usable.
				_ = err
			}
		}
		curX += logoSize + logoGap
	}

	// ── Text (vertically centred) ────────────────────────────────────────
	textGapPt := mmToPt(headerTextGapMM)
	var totalTextH float64
	if schoolName != "" {
		totalTextH += schoolNameFontPt + textGapPt
	}
	totalTextH += titleFontPt

	textY := y + (headerH-totalTextH)/2 + schoolNameFontPt - mmToPt(0.5)

	textX := curX
	if !hasLogo {
		textX = x + w/2
	}

	if schoolName != "" {
		_ = pdf.SetFont(fontBold, "", schoolNameFontPt)
		pdf.SetTextColor(60, 70, 80)
		if hasLogo {
			pdf.SetXY(textX, textY)
		} else {
			pdf.SetXY(textX-schoolNameW/2, textY)
		}
		pdf.Text(schoolName)
		textY += textGapPt + titleFontPt
	}

	_ = pdf.SetFont(fontBold, "", titleFontPt)
	pdf.SetTextColor(40, 50, 60)
	if hasLogo {
		pdf.SetXY(textX, textY)
	} else {
		pdf.SetXY(textX-titleW/2, textY)
	}
	pdf.Text(title)
}

// ---------------------------------------------------------------------------
// Primitive field helpers
// ---------------------------------------------------------------------------

// drawFieldLabel renders a small uppercase label.
func drawFieldLabel(pdf *gopdf.GoPdf, x, y float64, label string) {
	_ = pdf.SetFont(fontBold, "", 5)
	pdf.SetTextColor(100, 110, 120)
	pdf.SetXY(x, y)
	pdf.Text(label)
}

// drawFieldRow draws a label + bold value pair and returns the Y after the
// value row.  Values that exceed maxW are truncated.
func drawFieldRow(pdf *gopdf.GoPdf, x, y, maxW float64, label, value string, fontSize float64) float64 {
	drawFieldLabel(pdf, x, y, label)
	y += mmToPt(pdfLabelHMM)

	_ = pdf.SetFont(fontBold, "", fontSize)
	pdf.SetTextColor(30, 40, 50)
	pdf.SetXY(x, y)

	// Truncate if the value overflows the available width.
	for len(value) > 0 {
		tw, _ := pdf.MeasureTextWidth(value)
		if tw <= maxW {
			break
		}
		value = value[:len(value)-1]
	}
	pdf.Text(value)

	return y + mmToPt(pdfValueHMM)
}
