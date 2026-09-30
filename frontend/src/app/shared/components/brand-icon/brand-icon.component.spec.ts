import { ComponentFixture, TestBed } from '@angular/core/testing';
import { BrandIconComponent } from './brand-icon.component';

describe('BrandIconComponent', () => {
    let component: BrandIconComponent;
    let fixture: ComponentFixture<BrandIconComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [BrandIconComponent]
        }).compileComponents();

        fixture = TestBed.createComponent(BrandIconComponent);
        component = fixture.componentInstance;
        fixture.detectChanges();
    });

    it('should create and render the BayesMarket SVG emblem', () => {
        expect(component).toBeTruthy();
        const svg = fixture.nativeElement.querySelector('svg');
        expect(svg).toBeTruthy();
    });

    it('should render correct size dimensions', () => {
        fixture.componentRef.setInput('size', 'lg');
        fixture.detectChanges();
        const wrapper = fixture.nativeElement.querySelector('.brand-icon-box');
        expect(wrapper.style.width).toBe('44px');
        expect(wrapper.style.height).toBe('44px');
    });

    it('should apply glow class when glow input is true', () => {
        fixture.componentRef.setInput('glow', true);
        fixture.detectChanges();
        const wrapper = fixture.nativeElement.querySelector('.brand-icon-box');
        expect(wrapper.classList.contains('has-glow')).toBe(true);
    });
});
