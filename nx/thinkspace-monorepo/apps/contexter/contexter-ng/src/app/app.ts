import { Component } from '@angular/core';
import { LayoutComponent } from '@org/contexter-ui'; // adjust import path to match your Nx workspace setup

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [LayoutComponent],
  templateUrl: './app.html'
})
export class App {}