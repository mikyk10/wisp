WISP   = ../../../api/wisp
OUTDIR = ../../docs/defaults
TMPDIR = /tmp/wisp-defaults

DISPLAY_epd7in3e  = ws7in3e
DISPLAY_epd4in0e  = ws4in0e
DISPLAY_epd13in3e = ws13in3e

SIZE_epd7in3e  = 600 400
SIZE_epd4in0e  = 600 400
SIZE_epd13in3e = 1200 1600

DEVICES = epd7in3e epd4in0e epd13in3e

# Uncomment following if you wish to check the .png result
.PRECIOUS: $(TMPDIR)/%.png

.PHONY: all clean
all: $(addprefix $(OUTDIR)/,$(addsuffix .bin,$(DEVICES)))

$(TMPDIR):
	mkdir -p $@ $(OUTDIR)

$(TMPDIR)/%.png: src/%.html | $(TMPDIR)
	node capture.mjs $< $(SIZE_$*) $@

$(OUTDIR)/%.bin: $(TMPDIR)/%.png
	$(WISP) image convert $< --display $(DISPLAY_$*) --output $@

clean:
	rm -rf $(TMPDIR)
