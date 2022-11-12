# Dudirekta Protocol

**Unidirectional:**

```plaintext
< ["exampleReturnStringAndError"]
> ["1", "exampleReturnStringAndError", ["Hello, world!", false]]
< ["1","Hello, world!",""]
> ["1", "exampleReturnStringAndError", ["Hello, world!", true]]
< ["1","","test error"]
```

**Bidirectional:**

```plaintext
> [true, "1", "exampleReturnStringAndError", ["Hello, world!", false]]
< [false, "1","Hello, world!",""]
< [true, "1", "exampleReturnStringAndError", ["Hello, world!", true]]
> [false, "1","","test error"]
```
