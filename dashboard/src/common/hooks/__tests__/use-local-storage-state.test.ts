import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useLocalStorageState } from "../use-local-storage-state";

describe("useLocalStorageState", () => {
  // Save original localStorage
  const originalLocalStorage = global.localStorage;

  beforeEach(() => {
    // Reset localStorage before each test
    global.localStorage = {
      ...originalLocalStorage,
      getItem: vi.fn(),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
      length: 0,
      key: vi.fn(),
    } as Storage;

    // Clear all mocks
    vi.clearAllMocks();
  });

  afterEach(() => {
    // Restore original localStorage
    global.localStorage = originalLocalStorage;
  });

  describe("initialization", () => {
    it("should return default value when localStorage is empty", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default-value"),
      );

      expect(result.current[0]).toBe("default-value");
      expect(localStorage.getItem).toHaveBeenCalledWith("test-key");
    });

    it("should return stored value when localStorage has data", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify("stored-value"),
      );

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default-value"),
      );

      expect(result.current[0]).toBe("stored-value");
    });

    it("should handle complex objects", () => {
      const storedObject = { name: "John", age: 30 };
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify(storedObject),
      );

      const { result } = renderHook(() =>
        useLocalStorageState("user", { name: "", age: 0 }),
      );

      expect(result.current[0]).toEqual(storedObject);
    });

    it("should handle boolean values", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify(true),
      );

      const { result } = renderHook(() => useLocalStorageState("flag", false));

      expect(result.current[0]).toBe(true);
    });

    it("should return default value on deserialization error", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        "invalid-json",
      );

      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {});

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default-value"),
      );

      expect(result.current[0]).toBe("default-value");
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        'Failed to read localStorage key "test-key":',
        expect.any(Error),
      );

      consoleWarnSpy.mockRestore();
    });

    it("should return default value when localStorage throws error", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockImplementation(
        () => {
          throw new Error("localStorage unavailable");
        },
      );

      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {});

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default-value"),
      );

      expect(result.current[0]).toBe("default-value");
      expect(consoleWarnSpy).toHaveBeenCalled();

      consoleWarnSpy.mockRestore();
    });
  });

  describe("setValue", () => {
    it("should update state and localStorage", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default"),
      );

      act(() => {
        result.current[1]("new-value");
      });

      expect(result.current[0]).toBe("new-value");
      expect(localStorage.setItem).toHaveBeenCalledWith(
        "test-key",
        JSON.stringify("new-value"),
      );
    });

    it("should support functional updates", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify(5),
      );

      const { result } = renderHook(() => useLocalStorageState("counter", 0));

      act(() => {
        result.current[1]((prev) => prev + 1);
      });

      expect(result.current[0]).toBe(6);
      expect(localStorage.setItem).toHaveBeenCalledWith(
        "counter",
        JSON.stringify(6),
      );
    });

    it("should handle complex object updates", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const { result } = renderHook(() =>
        useLocalStorageState("user", { name: "", age: 0 }),
      );

      act(() => {
        result.current[1]({ name: "John", age: 30 });
      });

      expect(result.current[0]).toEqual({ name: "John", age: 30 });
      expect(localStorage.setItem).toHaveBeenCalledWith(
        "user",
        JSON.stringify({ name: "John", age: 30 }),
      );
    });

    it("should handle localStorage write errors gracefully", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);
      (localStorage.setItem as ReturnType<typeof vi.fn>).mockImplementation(
        () => {
          throw new Error("Quota exceeded");
        },
      );

      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {});

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default"),
      );

      act(() => {
        result.current[1]("new-value");
      });

      // State should still update even if localStorage fails
      expect(result.current[0]).toBe("new-value");
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        'Failed to write localStorage key "test-key":',
        expect.any(Error),
      );

      consoleWarnSpy.mockRestore();
    });
  });

  describe("removeValue", () => {
    it("should remove from localStorage and reset to default", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify("stored-value"),
      );

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default-value"),
      );

      act(() => {
        result.current[2](); // Call removeValue
      });

      expect(result.current[0]).toBe("default-value");
      expect(localStorage.removeItem).toHaveBeenCalledWith("test-key");
    });

    it("should handle localStorage remove errors gracefully", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);
      (localStorage.removeItem as ReturnType<typeof vi.fn>).mockImplementation(
        () => {
          throw new Error("localStorage unavailable");
        },
      );

      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {});

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default"),
      );

      act(() => {
        result.current[2]();
      });

      expect(result.current[0]).toBe("default");
      expect(consoleWarnSpy).toHaveBeenCalledWith(
        'Failed to remove localStorage key "test-key":',
        expect.any(Error),
      );

      consoleWarnSpy.mockRestore();
    });
  });

  describe("custom serialization", () => {
    it("should use custom serializer", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const { result } = renderHook(() =>
        useLocalStorageState("date", new Date("2024-01-01"), {
          serializer: (d) => d.toISOString(),
          deserializer: (s) => new Date(s),
        }),
      );

      const newDate = new Date("2024-12-31");

      act(() => {
        result.current[1](newDate);
      });

      expect(localStorage.setItem).toHaveBeenCalledWith(
        "date",
        newDate.toISOString(),
      );
    });

    it("should use custom deserializer", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        "2024-01-01T00:00:00.000Z",
      );

      const { result } = renderHook(() =>
        useLocalStorageState("date", new Date(), {
          serializer: (d) => d.toISOString(),
          deserializer: (s) => new Date(s),
        }),
      );

      expect(result.current[0]).toEqual(new Date("2024-01-01T00:00:00.000Z"));
    });
  });

  describe("cross-tab synchronization", () => {
    it("should sync state across tabs via storage event", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default", { syncAcrossTabs: true }),
      );

      // Simulate storage event from another tab
      const storageEvent = new StorageEvent("storage", {
        key: "test-key",
        newValue: JSON.stringify("value-from-other-tab"),
        oldValue: null,
      });

      act(() => {
        window.dispatchEvent(storageEvent);
      });

      expect(result.current[0]).toBe("value-from-other-tab");
    });

    it("should handle storage event with null newValue", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify("initial"),
      );

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default", { syncAcrossTabs: true }),
      );

      // Simulate removal from another tab
      const storageEvent = new StorageEvent("storage", {
        key: "test-key",
        newValue: null,
        oldValue: JSON.stringify("initial"),
      });

      act(() => {
        window.dispatchEvent(storageEvent);
      });

      expect(result.current[0]).toBe("default");
    });

    it("should ignore storage events for different keys", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(
        JSON.stringify("initial"),
      );

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default", { syncAcrossTabs: true }),
      );

      const storageEvent = new StorageEvent("storage", {
        key: "other-key",
        newValue: JSON.stringify("other-value"),
        oldValue: null,
      });

      act(() => {
        window.dispatchEvent(storageEvent);
      });

      expect(result.current[0]).toBe("initial");
    });

    it("should dispatch custom events when syncAcrossTabs is enabled", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const dispatchEventSpy = vi.spyOn(window, "dispatchEvent");

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default", { syncAcrossTabs: true }),
      );

      act(() => {
        result.current[1]("new-value");
      });

      expect(dispatchEventSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          type: "local-storage-change",
          detail: { key: "test-key", value: "new-value" },
        }),
      );

      dispatchEventSpy.mockRestore();
    });

    it("should not dispatch custom events when syncAcrossTabs is disabled", () => {
      (localStorage.getItem as ReturnType<typeof vi.fn>).mockReturnValue(null);

      const dispatchEventSpy = vi.spyOn(window, "dispatchEvent");

      const { result } = renderHook(() =>
        useLocalStorageState("test-key", "default", { syncAcrossTabs: false }),
      );

      act(() => {
        result.current[1]("new-value");
      });

      expect(dispatchEventSpy).not.toHaveBeenCalled();

      dispatchEventSpy.mockRestore();
    });
  });
});
