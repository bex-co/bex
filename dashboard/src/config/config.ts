interface Config {
  apiUrl: string;
  ssrApiUrl: string;
}

const apiUrl = import.meta.env.VITE_API_URL;
const ssrApiUrl = import.meta.env.VITE_SSR_API_URL || apiUrl;

if (import.meta.env.DEV && !apiUrl) {
  console.warn("[Config] VITE_API_URL is not set!");
}

export const config: Config = {
  apiUrl,
  ssrApiUrl,
};
