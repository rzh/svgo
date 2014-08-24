// benchviz: visualize benchmark data from benchcmp
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ajstarks/svgo"
)

// geometry defines the layout of the visualization
type geometry struct {
	top, left, width, height, vwidth, vp, barHeight int
	dolines, coldata, coltitle, coltitle_not_print  bool
	title, rcolor, scolor, style                    string
	deltamax, speedupmax                            float64
}

// process reads the input and calls the visualization function
func process(canvas *svg.SVG, filename string, g geometry) {
	if filename == "" {
		g.visualize(canvas, filename, os.Stdin)
		return
	}
	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	g.visualize(canvas, filename, f)
	f.Close()
}

// vmap maps world to canvas coordinates
func vmap(value, low1, high1, low2, high2 float64) float64 {
	return low2 + (high2-low2)*(value-low1)/(high1-low1)
}

// visualize performs the visualization of the input, reading a line a time
func (g *geometry) visualize(canvas *svg.SVG, filename string, f io.Reader) {
	var (
		err               error
		line, vs, bmtitle string
		dmin, dmax        float64
	)

	bh := g.barHeight
	vizwidth := g.vwidth
	vspacing := g.barHeight + (g.barHeight / 3) // vertical spacing
	bmtype := "delta"

	in := bufio.NewReader(f)
	canvas.Gstyle(fmt.Sprintf("font-size:%dpx;font-family:sans-serif", bh))
	canvas.Rect(0, 0, g.width, g.height, "stroke:lightgray;stroke-width:1;fill:white")
	if g.title == "" {
		bmtitle = filename
	} else {
		bmtitle = g.title
	}
	canvas.Text(g.left, g.top, bmtitle, "font-size:150%")
	for x, y, nr := g.left+g.vp, g.top+vspacing, 0; err == nil; nr++ {
		line, err = in.ReadString('\n')
		fields := strings.Fields(strings.TrimSpace(line))

		if len(fields) <= 1 || len(line) < 2 {
			continue
		}
		name := fields[0]
		value_old := fields[1]
		value_new := fields[2]
		value := fields[len(fields)-1]
		if len(value) > 2 {
			vs = value[:len(value)-1]
		}
		v, _ := strconv.ParseFloat(vs, 64)
		v = v * -1 // RUI, deal with delta for ops/s
		av := math.Abs(v)

		switch {
		case strings.HasPrefix(value, "delt"):
			bmtype = "delta"
			dmin = 0.0
			dmax = g.deltamax // 100.0
			y += vspacing * 2
			continue

		case strings.HasPrefix(value, "speed"):
			bmtype = "speedup"
			dmin = 0.0
			dmax = g.speedupmax // 10.0
			y += vspacing * 2
			continue

		case strings.HasPrefix(name, "#"):
			y += int(float64(vspacing) * 0.7)
			canvas.Text(g.left, y, line[1:], "font-style:italic;fill:gray;font-size:70%")
			continue
		}

		bw := int(vmap(av, dmin, dmax, 0, float64(vizwidth)))
		switch g.style {
		case "bar":
			g.bars(canvas, x, y, bw, bh, vspacing/2, bmtype, name, value, value_old, value_new, v)
		case "inline":
			g.inline(canvas, g.left, y, bw, bh, bmtype, name, value, v)
		default:
			g.bars(canvas, x, y, bw, bh, vspacing/2, bmtype, name, value, value_old, value_new, v)
		}
		y += vspacing
	}
	canvas.Gend()
}

// inline makes the inline style pf visualization
func (g *geometry) inline(canvas *svg.SVG, x, y, w, h int, bmtype, name, value string, v float64) {
	var color string
	switch bmtype {
	case "delta":
		if v > 0 {
			color = g.rcolor
		} else {
			color = g.scolor
		}
	case "speedup":
		if v < 1.0 {
			color = g.rcolor
		} else {
			color = g.scolor
		}
	}
	canvas.Text(x-10, y, value, "text-anchor:end")
	canvas.Text(x, y, name)
	canvas.Rect(x, y-h, w, h, "fill-opacity:0.3;fill:"+color)
}

// bars creates barchart style visualization
func (g *geometry) bars(canvas *svg.SVG, x, y, w, h, vs int, bmtype, name, value, value_old, value_new string, v float64) {
	canvas.Gstyle("font-style:italic;font-size:65%")
	toffset := h / 4
	var tx int
	var tstyle string
	switch bmtype {
	case "delta":
		if v > 0 {
			canvas.Rect(x-w, y-h/2, w, h, "fill-opacity:0.3;fill:"+g.rcolor)
			tx = x - w - toffset
			tstyle = "text-anchor:end"
		} else {
			canvas.Rect(x, y-h/2, w, h, "fill-opacity:0.3;fill:"+g.scolor)
			tx = x + w + toffset
			tstyle = "text-anchor:start"
		}
	case "speedup":
		if v < 1.0 {
			canvas.Rect(x-w, y-h/2, w, h, "fill-opacity:0.3;fill:"+g.rcolor)
			tx = x - w - toffset
			tstyle = "text-anchor:end"
		} else {
			canvas.Rect(x, y-h/2, w, h, "fill-opacity:0.3;fill:"+g.scolor)
			tx = x + w + toffset
			tstyle = "text-anchor:start"
		}
	}
	if g.coldata {
		canvas.Text(x-toffset, y+toffset, value, "text-anchor:end")
	} else {
		canvas.Text(tx, y+toffset, value, tstyle)
	}
	canvas.Gend()
	canvas.Text(g.left, y+(h/2), name, "text-anchor:start;font-size:70%")

	// print old & new value

	findColor := func(st string) string {
		re := regexp.MustCompile("([0-9.]+)%")
		f, e := strconv.ParseFloat(re.FindStringSubmatch(st)[1], 64)

		if e == nil && f > 2.0 {
			return ";fill:orange"
		}
		return ""
	}

	canvas.Text(g.left+325, y+(h/2), strings.Replace(value_old, "[", " [", 1), "text-anchor:start;font-size:55%"+findColor(value_old))
	canvas.Text(g.left+430, y+(h/2), strings.Replace(value_new, "[", " [", 1), "text-anchor:start;font-size:55%"+findColor(value_new))
	if g.dolines {
		canvas.Line(g.left, y+vs, g.left+(g.width-g.left), y+vs, "stroke:lightgray;stroke-width:1")
	}

	if g.coltitle && g.coltitle_not_print {
		canvas.Text(g.left, y-(h/2), "TestCase", "text-anchor:start;font-size:70%;font-weight:bold")

		// print old & new value
		canvas.Text(g.left+325, y-(h/2), "Baseline", "text-anchor:start;font-size:70%;font-weight:bold")
		canvas.Text(g.left+430, y-(h/2), "Test Data", "text-anchor:start;font-size:70%;font-weight:bold")
		g.coltitle_not_print = false
	}
}

func main() {
	var (
		width        = flag.Int("w", 1024, "width")
		height       = flag.Int("h", 768, "height")
		top          = flag.Int("top", 50, "top")
		left         = flag.Int("left", 100, "left margin")
		vp           = flag.Int("vp", 512, "visualization point")
		vw           = flag.Int("vw", 300, "visual area width")
		bh           = flag.Int("bh", 20, "bar height")
		smax         = flag.Float64("sm", 10, "maximum speedup")
		dmax         = flag.Float64("dm", 100, "maximum delta")
		title        = flag.String("title", "", "title")
		speedcolor   = flag.String("scolor", "green", "speedup color")
		regresscolor = flag.String("rcolor", "red", "regression color")
		style        = flag.String("style", "bar", "set the style (bar or inline)")
		lines        = flag.Bool("line", false, "show lines between entries")
		coldata      = flag.Bool("col", false, "show data in a single column")
		coltitle     = flag.Bool("coltitle", true, "show title for columns")
	)
	flag.Parse()

	g := geometry{
		width:              *width,
		height:             *height,
		top:                *top,
		left:               *left,
		vp:                 *vp,
		vwidth:             *vw,
		barHeight:          *bh,
		title:              *title,
		scolor:             *speedcolor,
		rcolor:             *regresscolor,
		style:              *style,
		dolines:            *lines,
		coldata:            *coldata,
		speedupmax:         *smax,
		deltamax:           *dmax,
		coltitle:           *coltitle,
		coltitle_not_print: true,
	}
	canvas := svg.New(os.Stdout)
	canvas.Start(g.width, g.height)
	if len(flag.Args()) > 0 {
		for _, f := range flag.Args() {
			process(canvas, f, g)
		}
	} else {
		process(canvas, "", g)
	}
	canvas.End()
}
