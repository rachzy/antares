import { Component } from '@angular/core';
import { RouterModule } from '@angular/router';
import { HlmButton } from '@spartan-ng/helm/button';

@Component({
  imports: [HlmButton, RouterModule],
  selector: 'an-root',
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {}
