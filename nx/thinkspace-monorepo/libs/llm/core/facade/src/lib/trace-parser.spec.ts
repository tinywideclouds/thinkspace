import { describe, it, expect } from 'vitest';
import { parseVerificationTrace, hasStructuredTrace, extractAttemptPrefix } from './trace-parser';

describe('Trace Parser', () => {
  it('should detect structured traces', () => {
    expect(hasStructuredTrace('[PRE-VERIFY] goimports -w src/: FAILED')).toBe(true);
    expect(hasStructuredTrace('[Attempt 1] Error: [VERIFY] go test: SUCCESS')).toBe(true);
    expect(hasStructuredTrace('Standard error message without tags')).toBe(false);
  });

  it('should extract attempt prefixes correctly', () => {
    expect(extractAttemptPrefix('[Attempt 1] Error: Patcher failed')).toBe('[Attempt 1] Error: ');
    expect(extractAttemptPrefix('No prefix here')).toBe('');
  });

  it('should parse a complex multi-step trace into distinct objects', () => {
    const rawTrace = `[PRE-VERIFY] goimports -w src/: SUCCESS
[VERIFY] go build -C src ./...: FAILED
src/main.go:12: undefined: x
src/main.go:15: syntax error`;

    const steps = parseVerificationTrace(rawTrace);
    
    expect(steps.length).toBe(2);
    
    expect(steps[0].type).toBe('PRE-VERIFY');
    expect(steps[0].command).toBe('goimports -w src/');
    expect(steps[0].status).toBe('SUCCESS');
    expect(steps[0].output).toBe('');
    expect(steps[0].isBestEffort).toBe(true);

    expect(steps[1].type).toBe('VERIFY');
    expect(steps[1].command).toBe('go build -C src ./...');
    expect(steps[1].status).toBe('FAILED');
    expect(steps[1].output).toBe('src/main.go:12: undefined: x\nsrc/main.go:15: syntax error');
    expect(steps[1].isBestEffort).toBe(false);
  });

  it('should fallback to a SYSTEM step if no valid tags are found', () => {
    const rawTrace = `[Attempt 2] Error: Patcher failed to apply JSON`;
    const steps = parseVerificationTrace(rawTrace);
    
    expect(steps.length).toBe(1);
    expect(steps[0].type).toBe('SYSTEM');
    expect(steps[0].command).toBe('[Attempt 2] Error:');
    expect(steps[0].output).toBe('Patcher failed to apply JSON');
  });
});