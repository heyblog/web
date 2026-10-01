import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { builtinModules } from 'node:module';
import { dirname, join, relative, resolve } from 'node:path';
import test from 'node:test';

import ts from 'typescript';

const sourceRoot = resolve(import.meta.dirname, '../src');
const nodeModules = new Set(builtinModules.map((name) => name.replace(/^node:/, '')));

interface SourceImport {
  readonly name: string;
  readonly runtime: boolean;
}

async function sourceFiles(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map(async (entry) => {
      const path = join(directory, entry.name);
      return entry.isDirectory() ? sourceFiles(path) : path.endsWith('.ts') ? [path] : [];
    }),
  );
  return nested.flat();
}

function importsFrom(file: string, text: string): SourceImport[] {
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true);
  const imports: SourceImport[] = [];
  function visit(node: ts.Node): void {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      const clause = node.importClause;
      const bindings = clause?.namedBindings;
      const typeOnly =
        clause?.phaseModifier === ts.SyntaxKind.TypeKeyword ||
        (clause?.name === undefined &&
          bindings &&
          ts.isNamedImports(bindings) &&
          bindings.elements.length > 0 &&
          bindings.elements.every((element) => element.isTypeOnly));
      imports.push({ name: node.moduleSpecifier.text, runtime: !typeOnly });
      return;
    }
    if (
      ts.isExportDeclaration(node) &&
      node.moduleSpecifier &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      const bindings = node.exportClause;
      const typeOnly =
        node.isTypeOnly ||
        (bindings &&
          ts.isNamedExports(bindings) &&
          bindings.elements.length > 0 &&
          bindings.elements.every((element) => element.isTypeOnly));
      imports.push({ name: node.moduleSpecifier.text, runtime: !typeOnly });
      return;
    }
    if (ts.isCallExpression(node) && node.expression.kind === ts.SyntaxKind.ImportKeyword) {
      const argument = node.arguments[0];
      if (argument && ts.isStringLiteral(argument))
        imports.push({ name: argument.text, runtime: true });
    }
    ts.forEachChild(node, visit);
  }
  visit(source);
  return imports;
}

async function dependencyGraph() {
  const files = await sourceFiles(sourceRoot);
  const graph = new Map<string, readonly SourceImport[]>();
  await Promise.all(
    files.map(async (file) => {
      graph.set(file, importsFrom(file, await readFile(file, 'utf8')));
    }),
  );
  function target(file: string, name: string): string | undefined {
    if (!name.startsWith('.') && !name.startsWith('@/')) return undefined;
    const path = name.startsWith('@/')
      ? resolve(sourceRoot, name.slice(2))
      : resolve(dirname(file), name);
    return [path, `${path}.ts`, join(path, 'index.ts')].find((candidate) => graph.has(candidate));
  }
  return { graph, target };
}

test('API and integrations never depend on application, even through shared helpers', async () => {
  const { graph, target } = await dependencyGraph();
  for (const entry of graph.keys()) {
    const path = relative(sourceRoot, entry);
    if (!path.startsWith('api/') && !path.startsWith('integrations/')) continue;
    const seen = new Set<string>();
    function visit(file: string): void {
      if (seen.has(file)) return;
      seen.add(file);
      assert.equal(
        relative(sourceRoot, file).startsWith('application/'),
        false,
        `${path} reaches ${relative(sourceRoot, file)}`,
      );
      for (const dependency of graph.get(file) ?? []) {
        const next = target(file, dependency.name);
        if (next) visit(next);
      }
    }
    visit(entry);
  }
});

test('browser TypeScript entry graphs exclude Node, private config, and server modules', async () => {
  const { graph, target } = await dependencyGraph();
  for (const entry of graph.keys()) {
    if (!entry.endsWith('.browser.ts') && !entry.endsWith('.svelte.ts')) continue;
    const seen = new Set<string>();
    function visit(file: string): void {
      if (seen.has(file)) return;
      seen.add(file);
      assert.equal(
        file.endsWith('.server.ts'),
        false,
        `${relative(sourceRoot, entry)} reaches ${relative(sourceRoot, file)}`,
      );
      for (const dependency of graph.get(file) ?? []) {
        if (!dependency.runtime) continue;
        assert.equal(
          dependency.name.startsWith('node:') || nodeModules.has(dependency.name),
          false,
          `${file} imports ${dependency.name}`,
        );
        const next = target(file, dependency.name);
        if (next) visit(next);
      }
    }
    visit(entry);
  }
});
