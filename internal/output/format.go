// Package output formats query results for display and export.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/nlink-jp/gem-query/internal/query"
)

// FormatTable renders a result as an ASCII table.
func FormatTable(w io.Writer, r *query.Result) {
	if len(r.Columns) == 0 {
		fmt.Fprintln(w, "(no columns)")
		return
	}

	// Calculate column widths
	widths := make([]int, len(r.Columns))
	for i, col := range r.Columns {
		widths[i] = len(col)
	}
	for _, row := range r.Rows {
		for i, val := range row {
			s := fmt.Sprintf("%v", val)
			if len(s) > widths[i] {
				widths[i] = len(s)
			}
		}
	}

	// Cap column width
	for i := range widths {
		if widths[i] > 40 {
			widths[i] = 40
		}
	}

	// Print header
	printSeparator(w, widths)
	printRow(w, r.Columns, widths)
	printSeparator(w, widths)

	// Print rows
	for _, row := range r.Rows {
		strs := make([]string, len(row))
		for i, val := range row {
			strs[i] = fmt.Sprintf("%v", val)
		}
		printRow(w, strs, widths)
	}
	printSeparator(w, widths)

	fmt.Fprintf(w, "%d rows\n", len(r.Rows))
}

// FormatJSON renders a result as a JSON array.
func FormatJSON(w io.Writer, r *query.Result) error {
	records := make([]map[string]any, len(r.Rows))
	for i, row := range r.Rows {
		record := make(map[string]any, len(r.Columns))
		for j, col := range r.Columns {
			record[col] = row[j]
		}
		records[i] = record
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

// FormatCSV renders a result as CSV.
func FormatCSV(w io.Writer, r *query.Result) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(r.Columns); err != nil {
		return err
	}

	for _, row := range r.Rows {
		record := make([]string, len(row))
		for i, val := range row {
			record[i] = fmt.Sprintf("%v", val)
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	return nil
}

func printSeparator(w io.Writer, widths []int) {
	fmt.Fprint(w, "+")
	for _, width := range widths {
		fmt.Fprintf(w, "%s+", strings.Repeat("-", width+2))
	}
	fmt.Fprintln(w)
}

func printRow(w io.Writer, values []string, widths []int) {
	fmt.Fprint(w, "|")
	for i, val := range values {
		if len(val) > widths[i] {
			val = val[:widths[i]-1] + "~"
		}
		fmt.Fprintf(w, " %-*s |", widths[i], val)
	}
	fmt.Fprintln(w)
}
