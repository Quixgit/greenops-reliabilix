/**
 * The sign-in scene made of two real photographs: a city of towers, softly out of focus at the top, that melts into a
 * green forest below. A slow zoom and drifting mist keep it alive. Decorative only (aria-hidden); the photos and their
 * licences are listed in docs/credits.md.
 */
export function LoginBackdrop() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden bg-[#10261a]">
      {/* forest: the whole scene underneath */}
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/login-forest.jpg" alt="" fetchPriority="high" className="photo-zoom absolute inset-0 h-full w-full object-cover [filter:saturate(1.05)_brightness(0.92)]" />

      {/* city: upper part, defocused, dissolving downwards into the forest */}
      <div className="absolute inset-x-0 top-0 h-[74%] [mask-image:linear-gradient(to_bottom,#000_0%,#000_52%,transparent_100%)] [-webkit-mask-image:linear-gradient(to_bottom,#000_0%,#000_52%,transparent_100%)]">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img src="/login-city.jpg" alt="" className="photo-zoom-slow h-full w-full object-cover object-bottom [filter:blur(3.5px)_saturate(1.05)_contrast(1.05)]" />
      </div>

      {/* grading: cool light on the city, deep green below, a calm patch behind the form */}
      <div className="absolute inset-0 bg-[linear-gradient(to_bottom,rgba(10,24,44,0.18)_0%,rgba(10,24,44,0.02)_40%,rgba(8,36,22,0.32)_100%)]" />
      <div className="absolute inset-0 bg-[radial-gradient(ellipse_at_50%_50%,rgba(255,255,255,0.12)_0%,transparent_50%)]" />

      {/* mist */}
      <div className="mist absolute -left-1/4 top-[46%] h-56 w-[150%] bg-[radial-gradient(ellipse_at_center,rgba(255,255,255,0.30)_0%,transparent_65%)]" />
      <div className="mist-slow absolute -left-1/3 top-[58%] h-64 w-[160%] bg-[radial-gradient(ellipse_at_center,rgba(230,255,240,0.22)_0%,transparent_65%)]" />
    </div>
  );
}
