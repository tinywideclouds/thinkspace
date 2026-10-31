import { Injectable } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class LoggerService {
  // Leverages Vite's environment variables to determine the build target
  private readonly isProd = import.meta.env.PROD;

  public info(context: string, message: string, ...args: any[]): void {
    if (this.isProd) return; 
    console.info(`ℹ️ [${context}] ${message}`, ...args);
  }

  public warn(context: string, message: string, ...args: any[]): void {
    console.warn(`⚠️ [${context}] ${message}`, ...args);
  }

  public error(context: string, message: string, error?: any): void {
    // Flat string output explicitly avoids console.group() DOM corruption
    console.error(`🚨 [${context}] ${message}`, error || '');
  }

  public success(context: string, message: string, ...args: any[]): void {
    if (this.isProd) return;
    console.log(`✅ [${context}] ${message}`, ...args);
  }
}