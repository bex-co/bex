import { describe, it, expect } from "vitest";
import { cn } from "../utils";

describe("Utils", () => {
  describe("cn", () => {
    it("should merge class names", () => {
      const result = cn("class1", "class2");
      expect(result).toBe("class1 class2");
    });

    it("should handle conditional classes", () => {
      const condition = false;
      const result = cn("base", condition && "conditional", "always");
      expect(result).toBe("base always");
    });

    it("should handle undefined and null values", () => {
      const result = cn("class1", undefined, null, "class2");
      expect(result).toBe("class1 class2");
    });

    it("should merge Tailwind classes with conflicts", () => {
      // twMerge should handle conflicting Tailwind classes
      const result = cn("px-2", "px-4");
      expect(result).toBe("px-4"); // Later class should win
    });

    it("should handle array of classes", () => {
      const result = cn(["class1", "class2"]);
      expect(result).toBe("class1 class2");
    });

    it("should handle object with boolean values", () => {
      const result = cn({
        class1: true,
        class2: false,
        class3: true,
      });
      expect(result).toBe("class1 class3");
    });

    it("should handle mixed types", () => {
      const result = cn(
        "base",
        ["array1", "array2"],
        { object1: true, object2: false },
        "final",
      );
      expect(result).toContain("base");
      expect(result).toContain("array1");
      expect(result).toContain("array2");
      expect(result).toContain("object1");
      expect(result).not.toContain("object2");
      expect(result).toContain("final");
    });

    it("should handle empty input", () => {
      const result = cn();
      expect(result).toBe("");
    });

    it("should handle only falsy values", () => {
      const result = cn(false, null, undefined, "");
      expect(result).toBe("");
    });

    it("should merge Tailwind color classes", () => {
      const result = cn("text-red-500", "text-blue-500");
      expect(result).toBe("text-blue-500");
    });

    it("should merge Tailwind spacing classes", () => {
      const result = cn("p-4", "p-2");
      expect(result).toBe("p-2");
    });

    it("should preserve non-conflicting Tailwind classes", () => {
      const result = cn("p-4", "m-2", "text-blue-500");
      expect(result).toContain("p-4");
      expect(result).toContain("m-2");
      expect(result).toContain("text-blue-500");
    });

    it("should handle nested arrays", () => {
      const result = cn([["nested1", "nested2"], "array1"]);
      expect(result).toContain("nested1");
      expect(result).toContain("nested2");
      expect(result).toContain("array1");
    });

    it("should handle conditional with ternary", () => {
      const isActive = true;
      const result = cn("base", isActive ? "active" : "inactive");
      expect(result).toContain("base");
      expect(result).toContain("active");
      expect(result).not.toContain("inactive");
    });

    it("should trim whitespace", () => {
      const result = cn("  class1  ", "  class2  ");
      expect(result).toBe("class1 class2");
    });

    it("should handle duplicate classes", () => {
      const result = cn("duplicate", "other", "duplicate");
      // Should preserve both non-conflicting duplicates in clsx
      expect(result).toContain("duplicate");
      expect(result).toContain("other");
    });

    it("should work with component variants pattern", () => {
      const variant: string = "primary";
      const size: string = "large";

      const result = cn(
        "base-class",
        {
          "variant-primary": variant === "primary",
          "variant-secondary": variant === "secondary",
        },
        {
          "size-small": size === "small",
          "size-large": size === "large",
        },
      );

      expect(result).toContain("base-class");
      expect(result).toContain("variant-primary");
      expect(result).not.toContain("variant-secondary");
      expect(result).not.toContain("size-small");
      expect(result).toContain("size-large");
    });

    it("should handle Tailwind responsive classes", () => {
      const result = cn("sm:p-2", "md:p-4", "lg:p-6");
      expect(result).toContain("sm:p-2");
      expect(result).toContain("md:p-4");
      expect(result).toContain("lg:p-6");
    });

    it("should handle Tailwind hover states", () => {
      const result = cn("hover:bg-blue-500", "hover:text-white");
      expect(result).toContain("hover:bg-blue-500");
      expect(result).toContain("hover:text-white");
    });

    it("should merge conflicting responsive classes", () => {
      const result = cn("sm:p-2", "sm:p-4");
      expect(result).toBe("sm:p-4");
    });
  });
});
