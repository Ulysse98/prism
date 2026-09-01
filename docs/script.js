const nav = document.querySelector('.nav-wrap');
const menuBtn = document.querySelector('.menu-btn');
const links = document.querySelector('.nav-links');
window.addEventListener('scroll', () => nav.classList.toggle('scrolled', scrollY > 20));
menuBtn.addEventListener('click', () => { const open = links.classList.toggle('open'); menuBtn.setAttribute('aria-expanded', open); });
document.querySelectorAll('.nav-links a').forEach(a => a.addEventListener('click', () => { links.classList.remove('open'); menuBtn.setAttribute('aria-expanded', 'false'); }));
const observer = new IntersectionObserver(entries => entries.forEach(e => { if (e.isIntersecting) e.target.classList.add('visible'); }), { threshold: .1 });
document.querySelectorAll('.reveal').forEach(el => observer.observe(el));
const ranges = ['stake','compute','community'];
function updateScore(){
  const vals = ranges.map(id => +document.getElementById(id).value);
  ranges.forEach((id,i) => document.getElementById(id+'V').textContent = vals[i]);
  const score = vals.reduce((a,b)=>a+b,0)/3;
  document.getElementById('score').textContent = score.toFixed(1);
  document.getElementById('scoreBar').style.width = score+'%';
  const max = Math.max(...vals), min = Math.min(...vals);
  document.getElementById('scoreText').textContent = max-min < 20 ? 'Contribution équilibrée sur les trois piliers.' : max===vals[0] ? 'Profil orienté sécurité et validation.' : max===vals[1] ? 'Profil orienté puissance de calcul utile.' : 'Profil orienté communauté et gouvernance.';
}
ranges.forEach(id => document.getElementById(id).addEventListener('input', updateScore));
