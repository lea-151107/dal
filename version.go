package dal

// Version identifies the rendering output. It changes whenever the output
// of Render, RenderText or RenderLine may change for the same input and
// options, so callers can use it as part of a cache key.
const Version = "0.4.14"
