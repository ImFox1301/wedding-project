/**
 * DateRangePicker — vanilla JS calendar with range selection.
 *
 * Usage:
 *   const picker = new DateRangePicker(containerEl, {
 *     min:       'YYYY-MM-DD',   // optional min date
 *     max:       'YYYY-MM-DD',   // optional max date
 *     from:      'YYYY-MM-DD',   // pre-selected start (ignored when fixedFrom is set)
 *     to:        'YYYY-MM-DD',   // pre-selected end
 *     fixedFrom: 'YYYY-MM-DD',   // fix the start date — user can only pick the end date
 *     labelFrom: 'Заезд',        // optional label for start prompt
 *     labelTo:   'Выезд',        // optional label for end prompt
 *     onChange:  (from, to) => {}// called when both dates are chosen
 *   });
 *
 *   picker.getValue()        → { from: 'YYYY-MM-DD', to: 'YYYY-MM-DD' }
 *   picker.setValue(from, to)
 *   picker.setRange(min, max)
 */
class DateRangePicker {
  constructor(el, opts = {}) {
    this.el        = el;
    this.minDate   = opts.min       ? this._parse(opts.min)       : null;
    this.maxDate   = opts.max       ? this._parse(opts.max)       : null;
    this.fixedFrom = opts.fixedFrom ? this._parse(opts.fixedFrom) : null;
    this.from      = this.fixedFrom || (opts.from ? this._parse(opts.from) : null);
    this.to        = opts.to        ? this._parse(opts.to)        : null;
    this.hover     = null;
    this.labelFrom = opts.labelFrom || 'Дата заезда';
    this.labelTo   = opts.labelTo   || 'Дата выезда';
    this.onChange  = opts.onChange  || (() => {});

    const base = this.from || this.minDate || new Date();
    this.view = new Date(base.getFullYear(), base.getMonth(), 1);

    this.el.classList.add('drp-wrap');

    // Attach all event listeners ONCE via delegation — no re-binding on re-render
    this._attachListeners();
    this._render();
  }

  _attachListeners() {
    // Single delegated click handler for the whole widget
    this.el.addEventListener('click', e => {
      // Navigation arrows
      const nav = e.target.closest('[data-dir]');
      if (nav) {
        const dir = parseInt(nav.dataset.dir);
        this.view = new Date(this.view.getFullYear(), this.view.getMonth() + dir, 1);
        this._render();
        return;
      }
      // Reset button
      if (e.target.closest('.drp-reset')) {
        this.from  = this.fixedFrom || null;
        this.to    = null;
        this.hover = null;
        this._render();
        this.onChange('', '');
        return;
      }
      // Date cells
      const cell = e.target.closest('[data-date]');
      if (cell && !cell.classList.contains('drp-disabled')) {
        this._pick(cell.dataset.date);
      }
    });

    // Delegated mouseover for hover-range preview
    this.el.addEventListener('mouseover', e => {
      const cell = e.target.closest('[data-date]');
      if (!cell || cell.classList.contains('drp-disabled')) return;
      const needHover = this.fixedFrom ? !this.to : (this.from && !this.to);
      if (needHover) {
        const h = this._parse(cell.dataset.date);
        if (!this.hover || h.getTime() !== this.hover.getTime()) {
          this.hover = h;
          this._render();
        }
      }
    });

    // Clear hover when mouse leaves the whole widget
    this.el.addEventListener('mouseleave', e => {
      if (!this.el.contains(e.relatedTarget) && this.hover) {
        this.hover = null;
        this._render();
      }
    });
  }

  _parse(str) {
    if (!str) return null;
    const d = new Date(str + 'T00:00:00');
    return isNaN(d) ? null : d;
  }

  _fmt(d) {
    if (!d) return '';
    const y   = d.getFullYear();
    const m   = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    return `${y}-${m}-${day}`;
  }

  _fmtDisplay(d) {
    if (!d) return '';
    return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' });
  }

  _eq(a, b) {
    return a && b && a.getTime() === b.getTime();
  }

  _render() {
    const MONTHS = ['Январь','Февраль','Март','Апрель','Май','Июнь',
                    'Июль','Август','Сентябрь','Октябрь','Ноябрь','Декабрь'];
    const DAYS   = ['Пн','Вт','Ср','Чт','Пт','Сб','Вс'];

    const yr  = this.view.getFullYear();
    const mo  = this.view.getMonth();
    const firstDow    = (new Date(yr, mo, 1).getDay() + 6) % 7; // Mon = 0
    const daysInMonth = new Date(yr, mo + 1, 0).getDate();

    // Effective range for highlighting (may use hover as preview end)
    const a = this.from;
    const b = this.to || this.hover;
    const rangeStart = a && b ? (a <= b ? a : b) : a;
    const rangeEnd   = a && b ? (a <= b ? b : a) : null;

    const today = new Date(); today.setHours(0, 0, 0, 0);

    // Build day cells (no event listeners — delegation handles everything)
    let cells = '';
    for (let i = 0; i < firstDow; i++) cells += '<div class="drp-cell drp-empty"></div>';

    for (let d = 1; d <= daysInMonth; d++) {
      const date    = new Date(yr, mo, d);
      const dateStr = this._fmt(date);
      let cls = 'drp-cell';

      const tooEarly    = this.minDate   && date <  this.minDate;
      const tooLate     = this.maxDate   && date >  this.maxDate;
      const beforeFixed = this.fixedFrom && date <= this.fixedFrom;

      if (tooEarly || tooLate || beforeFixed) {
        // Special case: show fixedFrom date itself as highlighted (non-clickable) range start
        if (this.fixedFrom && this._eq(date, this.fixedFrom)) {
          cls += ' drp-rs';
          cells += `<div class="${cls}">${d}</div>`;
        } else {
          cls += ' drp-disabled';
          cells += `<div class="${cls}">${d}</div>`;
        }
        continue;
      }

      // Highlight selected / hover range
      if (rangeStart && rangeEnd) {
        if      (this._eq(date, rangeStart))                    cls += ' drp-rs';
        else if (this._eq(date, rangeEnd))                      cls += ' drp-re';
        else if (date > rangeStart && date < rangeEnd)          cls += ' drp-in';
      } else if (a && this._eq(date, a)) {
        cls += ' drp-rs';
      }

      if (date.getTime() === today.getTime()) cls += ' drp-today';

      cells += `<div class="${cls}" data-date="${dateStr}">${d}</div>`;
    }

    // Footer hint
    let hint = '';
    if (this.fixedFrom) {
      if (!this.to) {
        hint = `<span class="drp-hint">Выберите ${this.labelTo.toLowerCase()}</span>`;
      } else {
        const nights = Math.round((this.to - this.from) / 86400000);
        hint = `<span class="drp-selected">${this._fmtDisplay(this.from)} → ${this._fmtDisplay(this.to)}</span>
                <span class="drp-nights">${nights} ${this._nightsWord(nights)}</span>`;
      }
    } else {
      if (!this.from) {
        hint = `<span class="drp-hint">Выберите ${this.labelFrom.toLowerCase()}</span>`;
      } else if (!this.to) {
        hint = `<span class="drp-hint">Выберите ${this.labelTo.toLowerCase()}</span>`;
      } else {
        const nights = Math.round((this.to - this.from) / 86400000);
        hint = `<span class="drp-selected">${this._fmtDisplay(this.from)} → ${this._fmtDisplay(this.to)}</span>
                <span class="drp-nights">${nights} ${this._nightsWord(nights)}</span>`;
      }
    }

    const canReset = this.fixedFrom ? !!this.to : !!(this.from || this.to);

    // Full re-render — no listeners added here; delegation on this.el handles everything
    this.el.innerHTML = `
      <div class="drp-header">
        <button type="button" class="drp-nav" data-dir="-1">&#8249;</button>
        <span class="drp-month">${MONTHS[mo]} ${yr}</span>
        <button type="button" class="drp-nav" data-dir="1">&#8250;</button>
      </div>
      <div class="drp-weekdays">${DAYS.map(w => `<div class="drp-wday">${w}</div>`).join('')}</div>
      <div class="drp-grid">${cells}</div>
      <div class="drp-footer">
        ${hint}
        ${canReset ? '<button type="button" class="drp-reset">Сбросить</button>' : ''}
      </div>
    `;
  }

  _pick(dateStr) {
    const date = this._parse(dateStr);

    if (this.fixedFrom) {
      // In fixed-start mode: every click sets the end date
      this.to    = date;
      this.hover = null;
    } else {
      if (!this.from || (this.from && this.to)) {
        // Start a new selection
        this.from  = date;
        this.to    = null;
        this.hover = null;
      } else {
        // Complete the range
        if (date < this.from) { this.to = this.from; this.from = date; }
        else                   { this.to = date; }
        this.hover = null;
      }
    }

    this._render();
    if (this.from && this.to) {
      this.onChange(this._fmt(this.from), this._fmt(this.to));
    }
  }

  _nightsWord(n) {
    const mod10 = n % 10, mod100 = n % 100;
    if (mod100 >= 11 && mod100 <= 19) return 'ночей';
    if (mod10 === 1)                   return 'ночь';
    if (mod10 >= 2 && mod10 <= 4)      return 'ночи';
    return 'ночей';
  }

  getValue() {
    return { from: this._fmt(this.from), to: this._fmt(this.to) };
  }

  setValue(from, to) {
    this.from  = from ? this._parse(from) : (this.fixedFrom || null);
    this.to    = to   ? this._parse(to)   : null;
    this.hover = null;
    if (this.from) {
      this.view = new Date(this.from.getFullYear(), this.from.getMonth(), 1);
    }
    this._render();
  }

  setRange(min, max) {
    this.minDate = min ? this._parse(min) : null;
    this.maxDate = max ? this._parse(max) : null;
    if (this.minDate && (!this.view || this.view < this.minDate)) {
      this.view = new Date(this.minDate.getFullYear(), this.minDate.getMonth(), 1);
    }
    this._render();
  }
}
