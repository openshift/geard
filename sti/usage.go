package sti

// Usage processes a build request by starting the container and executing
// the assemble script with a "-h" argument to print usage information
// for the script.
func Usage(req *STIRequest) error {
	h, err := newHandler(req)
	if err != nil {
		return err
	}
	defer h.cleanup()

	err = h.setup([]string{"usage"}, []string{}, "usage")
	if err != nil {
		return err
	}

	return h.execute("usage")
}
