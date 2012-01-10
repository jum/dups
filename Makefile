include $(GOROOT)/src/Make.inc

TARG=dups
GOFILES=dups.go

include $(GOROOT)/src/Make.cmd

GOFMT=gofmt
BADFMT=$(shell $(GOFMT) -l $(GOFILES) $(wildcard *_test.go))

gofmt: $(BADFMT)
	@for F in $(BADFMT); do $(GOFMT) -w $$F && echo $$F; done

ifneq ($(BADFMT),)
ifneq ($(MAKECMDGOALS),gofmt)
	$(warning WARNING: make gofmt: $(BADFMT))
endif
endif
