"""Measure rendered text contrast without adding a production dependency."""


def contrast_audit(page):
    return page.evaluate(r'''() => {
      const rgba = value => {
        if (!/^rgba?\(/.test(value)) return null;
        const values = value.match(/[\d.]+/g)?.map(Number);
        return values?.length >= 3 ? [...values.slice(0, 3), values[3] ?? 1] : null;
      };
      const over = (front, back) => front.slice(0, 3).map((v, i) => v * front[3] + back[i] * (1 - front[3]));
      const background = element => {
        const layers = [];
        for (let node = element; node; node = node.parentElement) {
          const style = getComputedStyle(node);
          if (Number(style.opacity) !== 1) return null;
          // The authored select chevron occupies the reserved right padding,
          // outside the text. Other background images need visual measurement.
          const sizes = style.backgroundSize.split(',').map(value => value.trim());
          const chevron = node.matches('select')
            // WebKit serializes the repeated layer size once; Chromium twice.
            && sizes.length <= 2 && sizes.every(value => value === '5px 5px')
            && style.backgroundPosition === 'calc(100% - 20px) 50%, calc(100% - 15px) 50%'
            && parseFloat(style.paddingRight) >= 40;
          if (style.backgroundImage !== 'none' && !chevron) return null;
          const color = rgba(style.backgroundColor);
          if (color) layers.push(color);
          if (color?.[3] === 1) break;
        }
        return layers.reverse().reduce((back, front) => over(front, back), [255, 255, 255]);
      };
      const luminance = rgb => rgb.map(v => v / 255).map(v => v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4).reduce((n, v, i) => n + v * [.2126, .7152, .0722][i], 0);
      const ratio = (a, b) => {
        const values = [luminance(a), luminance(b)].sort((a, b) => b - a);
        return (values[0] + .05) / (values[1] + .05);
      };
      const dialogs = [...document.querySelectorAll('dialog[open]')];
      const modal = dialogs.at(-1);
      const failures = [], pairs = new Map(), unmeasured = [];
      for (const element of document.querySelectorAll('body *')) {
        if (modal && !modal.contains(element)) continue;
        const style = getComputedStyle(element);
        if (!element.getClientRects().length || style.visibility !== 'visible' || element.closest('[hidden],[aria-hidden="true"],:disabled,script,style,option')) continue;
        const text = [...element.childNodes].filter(n => n.nodeType === Node.TEXT_NODE).map(n => n.textContent).join('').trim()
          || (element.matches('input,textarea') ? element.value : element.matches('select') ? element.selectedOptions[0]?.textContent : '');
        if (!text) continue;
        const back = background(element), front = rgba(style.color);
        if (!back || !front) { unmeasured.push({tag: element.tagName, image: style.backgroundImage, size: style.backgroundSize, position: style.backgroundPosition}); continue; }
        const size = parseFloat(style.fontSize), weight = parseFloat(style.fontWeight);
        const minimum = size >= 24 || size >= 18.66 && weight >= 700 ? 3 : 4.5;
        const actual = ratio(over(front, back), back);
        const pair = {foreground: style.color, background: back, minimum, ratio: Math.round(actual * 100) / 100};
        pairs.set(JSON.stringify(pair), pair);
        if (actual + .001 < minimum) failures.push({...pair, text: text.slice(0, 100), tag: element.tagName});
      }
      return {pairs: [...pairs.values()], failures, unmeasured};
    }''')
