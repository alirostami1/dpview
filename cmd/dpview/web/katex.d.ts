// Local type shim for the KaTeX auto-render helper used by the web app.
declare module "katex/contrib/auto-render" {
  export default function renderMathInElement(
    element: Element,
    options: {
      delimiters: Array<{ left: string; right: string; display: boolean }>;
      throwOnError: boolean;
    }
  ): void;
}
