const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');

const repoRoot = path.resolve(__dirname, '..', '..');
const frontendRequire = createRequire(
  path.join(repoRoot, 'flowai-studio-frontend', 'package.json'),
);
const ts = frontendRequire('typescript');

function getDecorators(node) {
  return ts.canHaveDecorators(node) ? (ts.getDecorators(node) || []) : [];
}

function getDecoratorCall(node, name) {
  return getDecorators(node)
    .map((decorator) => decorator.expression)
    .find(
      (expression) =>
        ts.isCallExpression(expression) &&
        ts.isIdentifier(expression.expression) &&
        expression.expression.text === name,
    );
}

function firstArgumentText(call) {
  if (!call || call.arguments.length === 0) return '';
  const argument = call.arguments[0];
  return ts.isStringLiteralLike(argument) ? argument.text : argument.getText();
}

function extractGuardNames(node) {
  const call = getDecoratorCall(node, 'UseGuards');
  return call ? call.arguments.map((argument) => argument.getText()) : [];
}

function joinApiPath(...parts) {
  const joined = parts.filter(Boolean).join('/').replace(/\/{2,}/g, '/');
  return joined.startsWith('/') ? joined : `/${joined}`;
}

function extractControllerRoutes(source, sourceName) {
  const file = ts.createSourceFile(
    sourceName,
    source,
    ts.ScriptTarget.Latest,
    true,
  );
  const routes = [];
  const verbs = [
    'Get',
    'Post',
    'Put',
    'Patch',
    'Delete',
    'Sse',
    'Options',
    'Head',
  ];

  file.forEachChild((node) => {
    if (!ts.isClassDeclaration(node)) return;
    const controller = getDecoratorCall(node, 'Controller');
    if (!controller) return;
    const prefix = firstArgumentText(controller);
    const classGuards = extractGuardNames(node);

    for (const member of node.members) {
      if (!ts.isMethodDeclaration(member)) continue;
      for (const verb of verbs) {
        const routeDecorator = getDecoratorCall(member, verb);
        if (!routeDecorator) continue;
        const position = file.getLineAndCharacterOfPosition(
          member.getStart(file),
        );
        routes.push({
          method: verb.toUpperCase(),
          path: joinApiPath(
            '/api',
            prefix,
            firstArgumentText(routeDecorator),
          ),
          guards: [
            ...new Set([...classGuards, ...extractGuardNames(member)]),
          ],
          source: sourceName,
          line: position.line + 1,
        });
      }
    }
  });

  return routes;
}

function pathFromExpression(expression) {
  if (ts.isStringLiteralLike(expression)) return expression.text;
  if (!ts.isTemplateExpression(expression)) return null;

  let result = expression.head.text;
  for (const span of expression.templateSpans) {
    result += `:${span.expression.getText()}` + span.literal.text;
  }
  return result;
}

function extractFrontendCalls(source, sourceName) {
  const file = ts.createSourceFile(
    sourceName,
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const calls = [];

  function visit(node) {
    if (ts.isCallExpression(node) && node.arguments.length > 0) {
      let method = null;

      if (ts.isIdentifier(node.expression) && node.expression.text === 'fetch') {
        method = 'GET';
        const options = node.arguments[1];
        if (options && ts.isObjectLiteralExpression(options)) {
          const property = options.properties.find(
            (item) =>
              ts.isPropertyAssignment(item) &&
              item.name.getText() === 'method',
          );
          if (
            property &&
            ts.isPropertyAssignment(property) &&
            ts.isStringLiteralLike(property.initializer)
          ) {
            method = property.initializer.text.toUpperCase();
          }
        }
      } else if (ts.isPropertyAccessExpression(node.expression)) {
        const owner = node.expression.expression.getText(file);
        const verb = node.expression.name.text.toUpperCase();
        if (
          owner === 'request' &&
          ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(verb)
        ) {
          method = verb;
        }
      }

      if (method) {
        const extracted = pathFromExpression(node.arguments[0]);
        if (extracted && extracted.startsWith('/')) {
          const position = file.getLineAndCharacterOfPosition(
            node.getStart(file),
          );
          calls.push({
            method,
            path: extracted.startsWith('/api/')
              ? extracted
              : joinApiPath('/api', extracted),
            expression: node.arguments[0].getText(file),
            source: sourceName,
            line: position.line + 1,
          });
        }
      }
    }

    ts.forEachChild(node, visit);
  }

  visit(file);
  return calls;
}

function normalizeComparablePath(value) {
  return value.replace(/:[^/]+/g, ':param').replace(/\/{2,}/g, '/');
}

function walkFiles(directory, predicate, output = []) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) walkFiles(absolute, predicate, output);
    else if (entry.isFile() && predicate(absolute)) output.push(absolute);
  }
  return output;
}

module.exports = {
  extractControllerRoutes,
  extractFrontendCalls,
  normalizeComparablePath,
  walkFiles,
};
