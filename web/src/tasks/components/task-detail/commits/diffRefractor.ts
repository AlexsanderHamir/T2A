import refractor from "refractor";
import bash from "refractor/lang/bash.js";
import c from "refractor/lang/c.js";
import cpp from "refractor/lang/cpp.js";
import csharp from "refractor/lang/csharp.js";
import css from "refractor/lang/css.js";
import docker from "refractor/lang/docker.js";
import go from "refractor/lang/go.js";
import ini from "refractor/lang/ini.js";
import java from "refractor/lang/java.js";
import javascript from "refractor/lang/javascript.js";
import json from "refractor/lang/json.js";
import markdown from "refractor/lang/markdown.js";
import markup from "refractor/lang/markup.js";
import python from "refractor/lang/python.js";
import ruby from "refractor/lang/ruby.js";
import rust from "refractor/lang/rust.js";
import sql from "refractor/lang/sql.js";
import toml from "refractor/lang/toml.js";
import tsx from "refractor/lang/tsx.js";
import typescript from "refractor/lang/typescript.js";
import yaml from "refractor/lang/yaml.js";
import { filePreviewLanguageFromPath } from "@/components/file-preview";

const languages = [
  bash,
  c,
  cpp,
  csharp,
  css,
  docker,
  go,
  ini,
  java,
  javascript,
  json,
  markdown,
  markup,
  python,
  ruby,
  rust,
  sql,
  toml,
  tsx,
  typescript,
  yaml,
];

let registered = false;

function ensureRefractorLanguages(): typeof refractor {
  if (!registered) {
    for (const lang of languages) {
      refractor.register(lang);
    }
    registered = true;
  }
  return refractor;
}

const prismToRefractor: Record<string, string> = {
  plain: "",
  markup: "markup",
  diff: "",
  git: "",
  docker: "docker",
};

export function refractorLanguageForPath(path: string): string | null {
  const { prism } = filePreviewLanguageFromPath(path);
  const mapped = prismToRefractor[prism] ?? prism;
  if (!mapped) {
    return null;
  }
  ensureRefractorLanguages();
  try {
    refractor.registered(mapped);
  } catch {
    return null;
  }
  return mapped;
}

export function getDiffRefractor(): typeof refractor {
  return ensureRefractorLanguages();
}
