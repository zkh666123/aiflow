const test = require('node:test');
const assert = require('node:assert/strict');
const {
  extractControllerRoutes,
  extractFrontendCalls,
  normalizeComparablePath,
} = require('./extract-contracts.cjs');

test('extracts controller method, full path, guard, and line', () => {
  const source = `
    @Controller('widgets')
    @UseGuards(JwtAuthGuard)
    export class WidgetController {
      @Get(':id')
      findOne() {}
    }
  `;

  assert.deepEqual(extractControllerRoutes(source, 'widget.controller.ts'), [
    {
      method: 'GET',
      path: '/api/widgets/:id',
      guards: ['JwtAuthGuard'],
      source: 'widget.controller.ts',
      line: 5,
    },
  ]);
});

test('extracts request and fetch calls with template parameters', () => {
  const source = `
    request.get('/apps');
    request.patch(\`/apps/\${id}\`, data);
    fetch(\`/api/workflows/\${workflowId}/run/stream\`, { method: 'POST' });
  `;

  assert.deepEqual(extractFrontendCalls(source, 'slice.ts'), [
    {
      method: 'GET',
      path: '/api/apps',
      expression: "'/apps'",
      source: 'slice.ts',
      line: 2,
    },
    {
      method: 'PATCH',
      path: '/api/apps/:id',
      expression: '`/apps/${id}`',
      source: 'slice.ts',
      line: 3,
    },
    {
      method: 'POST',
      path: '/api/workflows/:workflowId/run/stream',
      expression: '`/api/workflows/${workflowId}/run/stream`',
      source: 'slice.ts',
      line: 4,
    },
  ]);
});

test('normalizes parameter names for route comparison', () => {
  assert.equal(
    normalizeComparablePath('/api/apps/:appId/share'),
    '/api/apps/:param/share',
  );
});
