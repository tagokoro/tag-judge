pub fn output_matches(actual: &str, expected: &str) -> bool {
    actual.trim_end() == expected.trim_end()
}
