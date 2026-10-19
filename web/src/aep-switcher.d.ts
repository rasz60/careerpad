// ae:p 포털의 앱 전환 버튼 <aep-switcher> (포털이 /shell/switcher.js 로 제공하는 웹 컴포넌트)
import 'react';

declare module 'react' {
  namespace JSX {
    interface IntrinsicElements {
      'aep-switcher': React.DetailedHTMLProps<React.HTMLAttributes<HTMLElement>, HTMLElement> & {
        /** inline = 머리말 안에 놓이는 버튼, 없으면 화면 모서리에 떠 있는 버튼 */
        mode?: 'inline';
        position?: 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right';
        offset?: string;
      };
    }
  }
}
