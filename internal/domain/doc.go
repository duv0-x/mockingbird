// Package domain holds every entity, enum and value type of mockingbird.
//
// Owner: T0 (architect). Other tasks must not change field names, JSON tags
// or enum values without updating SPEC.md, because the JSON tags here ARE the
// admin REST API wire format consumed by the reused CastleMock frontend
// (SPEC.md §5). The on-disk format is NOT derived from these tags; it is owned
// by internal/storage/jsonfile (SPEC.md §3).
//
// Conventions mirroring the Java model (com.castlemock.model.*):
//   - Optional<String> + @JsonInclude(NON_EMPTY)  -> string with ",omitempty".
//   - Optional<Boolean>/Optional<Long> without NON_EMPTY -> pointer, serialized as null when absent.
//   - Lists and maps are never null on the wire: use EnsureSlices before encoding.
package domain
