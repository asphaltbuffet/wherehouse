# Clarifications

1. **ParseEventType location**: In `eventTypes.go` alongside the constants (all enum logic in one file).

2. **Mock update strategy**: Add a `//go:generate mockery` directive so the mock can be regenerated. Mockery is available in the toolchain.

3. **stringer tool**: Already available via mise. Use `mise run generate` to run all `go:generate` directives. Do NOT add it as a new dependency.

4. **Database storage**: Store as **string** in the database (for stability and human-readability). No int storage.

5. **ParseEventType implementation**: Use a map-based reverse lookup — no string duplication. The `// item.created` stringer comments are the single source of truth. The map is initialized using `.String()` calls on each constant:

   ```go
   var eventTypeByName = map[string]EventType{
       ItemCreatedEvent.String(): ItemCreatedEvent,
       // ...
   }

   func ParseEventType(s string) (EventType, error) {
       if et, ok := eventTypeByName[s]; ok {
           return et, nil
       }
       return 0, fmt.Errorf("unknown event type %q", s)
   }
   ```

   This means `ParseEventType` automatically stays in sync with stringer output — no separate string literals to maintain.
