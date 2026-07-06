import { describe, it, expect } from "vitest";
import {
  base64UrlEncode,
  base64UrlDecode,
  base64Encode,
  base64Decode,
} from "@/common/lib/utils/encode";

describe("Encode Utility", () => {
  describe("base64UrlEncode", () => {
    it("should encode a simple string", () => {
      const result = base64UrlEncode("hello");
      expect(result).toBe("aGVsbG8");
    });

    it("should replace + with - in base64 output", () => {
      // Characters that produce + in base64
      const input = String.fromCharCode(251, 239); // These bytes produce + in base64
      const result = base64UrlEncode(input);
      expect(result).not.toContain("+");
    });

    it("should replace / with _ in base64 output", () => {
      // Characters that produce / in base64
      const input = String.fromCharCode(255, 239); // These bytes produce / in base64
      const result = base64UrlEncode(input);
      expect(result).not.toContain("/");
    });

    it("should remove padding (=)", () => {
      const result = base64UrlEncode("hello world");
      expect(result).not.toContain("=");
    });

    it("should handle empty string", () => {
      const result = base64UrlEncode("");
      expect(result).toBe("");
    });

    it("should be URL-safe (no +, /, or =)", () => {
      const testStrings = [
        "hello world",
        "test@example.com",
        "user/path/to/resource",
        "a".repeat(100),
      ];

      testStrings.forEach((str) => {
        const result = base64UrlEncode(str);
        expect(result).not.toContain("+");
        expect(result).not.toContain("/");
        expect(result).not.toContain("=");
      });
    });
  });

  describe("base64UrlDecode", () => {
    it("should decode a simple base64url string", () => {
      const encoded = base64UrlEncode("hello");
      const result = base64UrlDecode(encoded);
      expect(result).toBe("hello");
    });

    it("should handle strings with - (plus replacement)", () => {
      const encoded = "aGVsbG8-d29ybGQ"; // contains -
      const result = base64UrlDecode(encoded);
      expect(result).toBeTruthy();
    });

    it("should handle strings with _ (slash replacement)", () => {
      const encoded = "aGVsbG8_d29ybGQ"; // contains _
      const result = base64UrlDecode(encoded);
      expect(result).toBeTruthy();
    });

    it("should add padding when needed", () => {
      // Test with different padding requirements
      const testCases = [
        base64UrlEncode("a"), // Will need padding
        base64UrlEncode("ab"),
        base64UrlEncode("abc"),
        base64UrlEncode("abcd"),
      ];

      testCases.forEach((encoded) => {
        const decoded = base64UrlDecode(encoded);
        expect(decoded).toBeTruthy();
      });
    });

    it("should round-trip encode/decode", () => {
      const original = "some/path-name_123";
      const encoded = base64UrlEncode(original);
      const decoded = base64UrlDecode(encoded);
      expect(decoded).toBe(original);
    });

    it("should handle empty string", () => {
      const result = base64UrlDecode("");
      expect(result).toBe("");
    });
  });

  describe("base64Encode (UTF-8 safe)", () => {
    it("should encode a simple ASCII string", () => {
      const result = base64Encode("hello world");
      expect(result).toBe("aGVsbG8gd29ybGQ=");
    });

    it("should handle empty string", () => {
      const result = base64Encode("");
      expect(result).toBe("");
    });

    it("should encode emoji characters", () => {
      const input = "Hello 🍕 World";
      const result = base64Encode(input);
      expect(result).toBeTruthy();
      expect(result.length).toBeGreaterThan(0);
      // Should not throw InvalidCharacterError
      expect(() => base64Encode(input)).not.toThrow();
    });

    it("should encode Chinese characters", () => {
      const input = "你好世界";
      const result = base64Encode(input);
      expect(result).toBeTruthy();
      expect(result.length).toBeGreaterThan(0);
      expect(() => base64Encode(input)).not.toThrow();
    });

    it("should encode mixed UTF-8 characters", () => {
      const input = "Café ☕ 咖啡馆 🍵";
      const result = base64Encode(input);
      expect(result).toBeTruthy();
      expect(() => base64Encode(input)).not.toThrow();
    });

    it("should encode special Unicode symbols", () => {
      const input = "→ ← ↑ ↓ ✓ ✗ ★ ♥ ♦ ♣ ♠";
      const result = base64Encode(input);
      expect(result).toBeTruthy();
      expect(() => base64Encode(input)).not.toThrow();
    });

    it("should handle newlines and tabs", () => {
      const input = "line1\nline2\tcolumn";
      const result = base64Encode(input);
      expect(result).toBeTruthy();
    });
  });

  describe("base64Decode (UTF-8 safe)", () => {
    it("should decode a simple ASCII string", () => {
      const encoded = base64Encode("hello world");
      const result = base64Decode(encoded);
      expect(result).toBe("hello world");
    });

    it("should handle empty string", () => {
      const result = base64Decode("");
      expect(result).toBe("");
    });

    it("should round-trip encode/decode emoji", () => {
      const original = "Hello 🍕 World";
      const encoded = base64Encode(original);
      const decoded = base64Decode(encoded);
      expect(decoded).toBe(original);
    });

    it("should round-trip encode/decode Chinese characters", () => {
      const original = "你好世界";
      const encoded = base64Encode(original);
      const decoded = base64Decode(encoded);
      expect(decoded).toBe(original);
    });

    it("should round-trip encode/decode mixed UTF-8", () => {
      const original = "Café ☕ 咖啡馆 🍵";
      const encoded = base64Encode(original);
      const decoded = base64Decode(encoded);
      expect(decoded).toBe(original);
    });

    it("should round-trip special Unicode symbols", () => {
      const original = "→ ← ↑ ↓ ✓ ✗ ★ ♥ ♦ ♣ ♠";
      const encoded = base64Encode(original);
      const decoded = base64Decode(encoded);
      expect(decoded).toBe(original);
    });

    it("should handle long strings", () => {
      const original = "test ".repeat(1000) + "🎉";
      const encoded = base64Encode(original);
      const decoded = base64Decode(encoded);
      expect(decoded).toBe(original);
    });
  });
});
