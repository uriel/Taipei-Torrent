include $(GOROOT)/src/Make.$(GOARCH)

all : taipeitorrent

# Is there a proper way to recurse into dht? This won't run clean there for example.
GC+=-I taipei/_obj
LD+=-L taipei/_obj
PREREQ=.taipei
.taipei:
	$(MAKE) -C taipei

CLEANFILES=taipei/*.o taipei/*.a taipei/_obj taipei/*.[$(OS)] [$(OS)].out

TARG=taipeitorrent
GOFILES=main.go

include $(GOROOT)/src/Make.cmd
