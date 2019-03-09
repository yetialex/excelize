// Copyright 2016 - 2019 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to
// and read from XLSX files. Support reads and writes XLSX file generated by
// Microsoft Excel™ 2007 and later. Support save file without losing original
// charts of XLSX. This library needs Go version 1.8 or later.

package excelize

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// GetRows return all the rows in a sheet by given worksheet name (case
// sensitive). For example:
//
//    for _, row := range xlsx.GetRows("Sheet1") {
//        for _, colCell := range row {
//            fmt.Print(colCell, "\t")
//        }
//        fmt.Println()
//    }
//
func (f *File) GetRows(sheet string) [][]string {
	xlsx := f.workSheetReader(sheet)
	name, ok := f.sheetMap[trimSheetName(sheet)]
	if !ok {
		return [][]string{}
	}
	if xlsx != nil {
		output, _ := xml.Marshal(f.Sheet[name])
		f.saveFileList(name, replaceWorkSheetsRelationshipsNameSpaceBytes(output))
	}
	xml.NewDecoder(bytes.NewReader(f.readXML(name)))
	d := f.sharedStringsReader()
	var inElement string
	var r xlsxRow
	tr, tc := f.getTotalRowsCols(name)
	rows := make([][]string, tr)
	for i := range rows {
		rows[i] = make([]string, tc+1)
	}
	var row int
	decoder := xml.NewDecoder(bytes.NewReader(f.readXML(name)))
	for {
		token, _ := decoder.Token()
		if token == nil {
			break
		}
		switch startElement := token.(type) {
		case xml.StartElement:
			inElement = startElement.Name.Local
			if inElement == "row" {
				r = xlsxRow{}
				_ = decoder.DecodeElement(&r, &startElement)
				cr := r.R - 1
				for _, colCell := range r.C {
					c := TitleToNumber(strings.Map(letterOnlyMapF, colCell.R))
					val, _ := colCell.getValueFrom(f, d)
					rows[cr][c] = val
					if val != "" {
						row = r.R
					}
				}
			}
		default:
		}
	}
	return rows[:row]
}

// Rows defines an iterator to a sheet
type Rows struct {
	decoder *xml.Decoder
	token   xml.Token
	err     error
	f       *File
}

// Next will return true if find the next row element.
func (rows *Rows) Next() bool {
	for {
		rows.token, rows.err = rows.decoder.Token()
		if rows.err == io.EOF {
			rows.err = nil
		}
		if rows.token == nil {
			return false
		}

		switch startElement := rows.token.(type) {
		case xml.StartElement:
			inElement := startElement.Name.Local
			if inElement == "row" {
				return true
			}
		}
	}
}

// Error will return the error when the find next row element
func (rows *Rows) Error() error {
	return rows.err
}

// Columns return the current row's column values
func (rows *Rows) Columns() []string {
	if rows.token == nil {
		return []string{}
	}
	startElement := rows.token.(xml.StartElement)
	r := xlsxRow{}
	_ = rows.decoder.DecodeElement(&r, &startElement)
	d := rows.f.sharedStringsReader()
	row := make([]string, len(r.C))
	for _, colCell := range r.C {
		c := TitleToNumber(strings.Map(letterOnlyMapF, colCell.R))
		val, _ := colCell.getValueFrom(rows.f, d)
		row[c] = val
	}
	return row
}

// ErrSheetNotExist defines an error of sheet is not exist
type ErrSheetNotExist struct {
	SheetName string
}

func (err ErrSheetNotExist) Error() string {
	return fmt.Sprintf("Sheet %s is not exist", string(err.SheetName))
}

// Rows return a rows iterator. For example:
//
//    rows, err := xlsx.Rows("Sheet1")
//    for rows.Next() {
//        for _, colCell := range rows.Columns() {
//            fmt.Print(colCell, "\t")
//        }
//        fmt.Println()
//    }
//
func (f *File) Rows(sheet string) (*Rows, error) {
	xlsx := f.workSheetReader(sheet)
	name, ok := f.sheetMap[trimSheetName(sheet)]
	if !ok {
		return nil, ErrSheetNotExist{sheet}
	}
	if xlsx != nil {
		output, _ := xml.Marshal(f.Sheet[name])
		f.saveFileList(name, replaceWorkSheetsRelationshipsNameSpaceBytes(output))
	}
	return &Rows{
		f:       f,
		decoder: xml.NewDecoder(bytes.NewReader(f.readXML(name))),
	}, nil
}

// getTotalRowsCols provides a function to get total columns and rows in a
// worksheet.
func (f *File) getTotalRowsCols(name string) (int, int) {
	decoder := xml.NewDecoder(bytes.NewReader(f.readXML(name)))
	var inElement string
	var r xlsxRow
	var tr, tc int
	for {
		token, _ := decoder.Token()
		if token == nil {
			break
		}
		switch startElement := token.(type) {
		case xml.StartElement:
			inElement = startElement.Name.Local
			if inElement == "row" {
				r = xlsxRow{}
				_ = decoder.DecodeElement(&r, &startElement)
				tr = r.R
				for _, colCell := range r.C {
					col := TitleToNumber(strings.Map(letterOnlyMapF, colCell.R))
					if col > tc {
						tc = col
					}
				}
			}
		default:
		}
	}
	return tr, tc
}

// SetRowHeight provides a function to set the height of a single row. For
// example, set the height of the first row in Sheet1:
//
//    xlsx.SetRowHeight("Sheet1", 1, 50)
//
func (f *File) SetRowHeight(sheet string, row int, height float64) {
	xlsx := f.workSheetReader(sheet)
	if row < 1 {
		return
	}
	cells := 0
	rowIdx := row - 1
	completeRow(xlsx, row, cells)
	xlsx.SheetData.Row[rowIdx].Ht = height
	xlsx.SheetData.Row[rowIdx].CustomHeight = true
}

// getRowHeight provides a function to get row height in pixels by given sheet
// name and row index.
func (f *File) getRowHeight(sheet string, row int) int {
	xlsx := f.workSheetReader(sheet)
	for _, v := range xlsx.SheetData.Row {
		if v.R == row+1 && v.Ht != 0 {
			return int(convertRowHeightToPixels(v.Ht))
		}
	}
	// Optimisation for when the row heights haven't changed.
	return int(defaultRowHeightPixels)
}

// GetRowHeight provides a function to get row height by given worksheet name
// and row index. For example, get the height of the first row in Sheet1:
//
//    xlsx.GetRowHeight("Sheet1", 1)
//
func (f *File) GetRowHeight(sheet string, row int) float64 {
	xlsx := f.workSheetReader(sheet)
	if row < 1 || row > len(xlsx.SheetData.Row) {
		return defaultRowHeightPixels // it will be better to use 0, but we take care with BC
	}
	for _, v := range xlsx.SheetData.Row {
		if v.R == row && v.Ht != 0 {
			return v.Ht
		}
	}
	// Optimisation for when the row heights haven't changed.
	return defaultRowHeightPixels
}

// sharedStringsReader provides a function to get the pointer to the structure
// after deserialization of xl/sharedStrings.xml.
func (f *File) sharedStringsReader() *xlsxSST {
	if f.SharedStrings == nil {
		var sharedStrings xlsxSST
		ss := f.readXML("xl/sharedStrings.xml")
		if len(ss) == 0 {
			ss = f.readXML("xl/SharedStrings.xml")
		}
		_ = xml.Unmarshal(namespaceStrictToTransitional(ss), &sharedStrings)
		f.SharedStrings = &sharedStrings
	}
	return f.SharedStrings
}

// getValueFrom return a value from a column/row cell, this function is
// inteded to be used with for range on rows an argument with the xlsx opened
// file.
func (xlsx *xlsxC) getValueFrom(f *File, d *xlsxSST) (string, error) {
	switch xlsx.T {
	case "s":
		xlsxSI := 0
		xlsxSI, _ = strconv.Atoi(xlsx.V)
		if len(d.SI[xlsxSI].R) > 0 {
			value := ""
			for _, v := range d.SI[xlsxSI].R {
				value += v.T
			}
			return value, nil
		}
		return f.formattedValue(xlsx.S, d.SI[xlsxSI].T), nil
	case "str":
		return f.formattedValue(xlsx.S, xlsx.V), nil
	case "inlineStr":
		return f.formattedValue(xlsx.S, xlsx.IS.T), nil
	default:
		return f.formattedValue(xlsx.S, xlsx.V), nil
	}
}

// SetRowVisible provides a function to set visible of a single row by given
// worksheet name and Excel row number. For example, hide row 2 in Sheet1:
//
//    xlsx.SetRowVisible("Sheet1", 2, false)
//
func (f *File) SetRowVisible(sheet string, row int, visible bool) {
	xlsx := f.workSheetReader(sheet)
	if row < 1 {
		return
	}
	cells := 0
	completeRow(xlsx, row, cells)
	rowIdx := row - 1
	if visible {
		xlsx.SheetData.Row[rowIdx].Hidden = false
		return
	}
	xlsx.SheetData.Row[rowIdx].Hidden = true
}

// GetRowVisible provides a function to get visible of a single row by given
// worksheet name and Excel row number. For example, get visible state of row
// 2 in Sheet1:
//
//    xlsx.GetRowVisible("Sheet1", 2)
//
func (f *File) GetRowVisible(sheet string, row int) bool {
	xlsx := f.workSheetReader(sheet)
	if row < 1 || row > len(xlsx.SheetData.Row) {
		return false
	}
	rowIndex := row - 1
	cells := 0
	completeRow(xlsx, row, cells)
	return !xlsx.SheetData.Row[rowIndex].Hidden
}

// SetRowOutlineLevel provides a function to set outline level number of a
// single row by given worksheet name and Excel row number. For example, outline row
// 2 in Sheet1 to level 1:
//
//    xlsx.SetRowOutlineLevel("Sheet1", 2, 1)
//
func (f *File) SetRowOutlineLevel(sheet string, row int, level uint8) {
	xlsx := f.workSheetReader(sheet)
	if row < 1 {
		return
	}
	cells := 0
	completeRow(xlsx, row, cells)
	xlsx.SheetData.Row[row-1].OutlineLevel = level
}

// GetRowOutlineLevel provides a function to get outline level number of a
// single row by given worksheet name and Exce row number.
// For example, get outline number of row 2 in Sheet1:
//
//    xlsx.GetRowOutlineLevel("Sheet1", 2)
//
func (f *File) GetRowOutlineLevel(sheet string, row int) uint8 {
	xlsx := f.workSheetReader(sheet)
	if row < 1 || row > len(xlsx.SheetData.Row) {
		return 0
	}
	return xlsx.SheetData.Row[row-1].OutlineLevel
}

// RemoveRow provides a function to remove single row by given worksheet name
// and Excel row number. For example, remove row 3 in Sheet1:
//
//    xlsx.RemoveRow("Sheet1", 3)
//
// Use this method with caution, which will affect changes in references such
// as formulas, charts, and so on. If there is any referenced value of the
// worksheet, it will cause a file error when you open it. The excelize only
// partially updates these references currently.
func (f *File) RemoveRow(sheet string, row int) {
	xlsx := f.workSheetReader(sheet)
	if row < 1 || row > len(xlsx.SheetData.Row) {
		return
	}
	for i, r := range xlsx.SheetData.Row {
		if r.R == row {
			xlsx.SheetData.Row = append(xlsx.SheetData.Row[:i], xlsx.SheetData.Row[i+1:]...)
			f.adjustHelper(sheet, -1, row, -1)
			return
		}
	}
}

// InsertRow provides a function to insert a new row after given Excel row
// number starting from 1. For example, create a new row before row 3 in
// Sheet1:
//
//    xlsx.InsertRow("Sheet1", 3)
//
func (f *File) InsertRow(sheet string, row int) {
	if row < 1 {
		return
	}
	f.adjustHelper(sheet, -1, row, 1)
}

// DuplicateRow inserts a copy of specified row (by it Excel row number) below
//
//    xlsx.DuplicateRow("Sheet1", 2)
//
// Use this method with caution, which will affect changes in references such
// as formulas, charts, and so on. If there is any referenced value of the
// worksheet, it will cause a file error when you open it. The excelize only
// partially updates these references currently.
func (f *File) DuplicateRow(sheet string, row int) {
	f.DuplicateRowTo(sheet, row, row+1)
}

// DuplicateRowTo inserts a copy of specified row by it Excel number
// to specified row position moving down exists rows after target position
//
//    xlsx.DuplicateRowTo("Sheet1", 2, 7)
//
// Use this method with caution, which will affect changes in references such
// as formulas, charts, and so on. If there is any referenced value of the
// worksheet, it will cause a file error when you open it. The excelize only
// partially updates these references currently.
func (f *File) DuplicateRowTo(sheet string, row, row2 int) {
	xlsx := f.workSheetReader(sheet)

	if row < 1 || row > len(xlsx.SheetData.Row) || row2 < 1 || row == row2 {
		return
	}

	var ok bool
	var rowCopy xlsxRow

	for i, r := range xlsx.SheetData.Row {
		if r.R == row {
			rowCopy = xlsx.SheetData.Row[i]
			ok = true
			break
		}
	}
	if !ok {
		return
	}

	f.adjustHelper(sheet, -1, row2, 1)

	idx2 := -1
	for i, r := range xlsx.SheetData.Row {
		if r.R == row2 {
			idx2 = i
			break
		}
	}
	if idx2 == -1 && len(xlsx.SheetData.Row) >= row2 {
		return
	}

	rowCopy.C = append(make([]xlsxC, 0, len(rowCopy.C)), rowCopy.C...)
	f.ajustSingleRowDimensions(&rowCopy, row2)

	if idx2 != -1 {
		xlsx.SheetData.Row[idx2] = rowCopy
	} else {
		xlsx.SheetData.Row = append(xlsx.SheetData.Row, rowCopy)
	}
}

// checkRow provides a function to check and fill each column element for all
// rows and make that is continuous in a worksheet of XML. For example:
//
//    <row r="15" spans="1:22" x14ac:dyDescent="0.2">
//        <c r="A15" s="2" />
//        <c r="B15" s="2" />
//        <c r="F15" s="1" />
//        <c r="G15" s="1" />
//    </row>
//
// in this case, we should to change it to
//
//    <row r="15" spans="1:22" x14ac:dyDescent="0.2">
//        <c r="A15" s="2" />
//        <c r="B15" s="2" />
//        <c r="C15" s="2" />
//        <c r="D15" s="2" />
//        <c r="E15" s="2" />
//        <c r="F15" s="1" />
//        <c r="G15" s="1" />
//    </row>
//
// Noteice: this method could be very slow for large spreadsheets (more than
// 3000 rows one sheet).
func checkRow(xlsx *xlsxWorksheet) {
	buffer := bytes.Buffer{}
	for k := range xlsx.SheetData.Row {
		lenCol := len(xlsx.SheetData.Row[k].C)
		if lenCol > 0 {
			endR := string(strings.Map(letterOnlyMapF, xlsx.SheetData.Row[k].C[lenCol-1].R))
			endRow, _ := strconv.Atoi(strings.Map(intOnlyMapF, xlsx.SheetData.Row[k].C[lenCol-1].R))
			endCol := TitleToNumber(endR) + 1
			if lenCol < endCol {
				oldRow := xlsx.SheetData.Row[k].C
				xlsx.SheetData.Row[k].C = xlsx.SheetData.Row[k].C[:0]
				var tmp []xlsxC
				for i := 0; i < endCol; i++ {
					buffer.WriteString(ToAlphaString(i))
					buffer.WriteString(strconv.Itoa(endRow))
					tmp = append(tmp, xlsxC{
						R: buffer.String(),
					})
					buffer.Reset()
				}
				xlsx.SheetData.Row[k].C = tmp
				for _, y := range oldRow {
					colAxis := TitleToNumber(string(strings.Map(letterOnlyMapF, y.R)))
					xlsx.SheetData.Row[k].C[colAxis] = y
				}
			}
		}
	}
}

// completeRow provides a function to check and fill each column element for a
// single row and make that is continuous in a worksheet of XML by given row
// index and axis.
func completeRow(xlsx *xlsxWorksheet, row, cell int) {
	currentRows := len(xlsx.SheetData.Row)

	if currentRows > 1 {
		lastRow := xlsx.SheetData.Row[currentRows-1].R
		if lastRow >= row {
			row = lastRow
		}
	}

	for i := currentRows; i < row; i++ {
		xlsx.SheetData.Row = append(xlsx.SheetData.Row, xlsxRow{
			R: i + 1,
		})
	}

	buffer := bytes.Buffer{}
	for ii := currentRows; ii < row; ii++ {
		start := len(xlsx.SheetData.Row[ii].C)
		if start == 0 {
			for iii := start; iii < cell; iii++ {
				buffer.WriteString(ToAlphaString(iii))
				buffer.WriteString(strconv.Itoa(ii + 1))
				xlsx.SheetData.Row[ii].C = append(xlsx.SheetData.Row[ii].C, xlsxC{
					R: buffer.String(),
				})
				buffer.Reset()
			}
		}
	}
}

// convertRowHeightToPixels provides a function to convert the height of a
// cell from user's units to pixels. If the height hasn't been set by the user
// we use the default value. If the row is hidden it has a value of zero.
func convertRowHeightToPixels(height float64) float64 {
	var pixels float64
	if height == 0 {
		return pixels
	}
	pixels = math.Ceil(4.0 / 3.0 * height)
	return pixels
}
