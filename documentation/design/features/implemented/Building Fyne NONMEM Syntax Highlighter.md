

# **Implementation Plan for a High-Performance Fyne Editor Canvas Component with NONMEM Syntax Highlighting**

This report details the comprehensive architectural and implementation strategy for developing a custom Fyne widget in Golang, designed specifically to serve as an advanced editor canvas providing syntax highlighting for NONMEM control streams. The plan prioritizes performance, idiomatic Golang practices, and robust integration with the Fyne rendering lifecycle.

---

## **I. Architectural Foundation: The Fyne Editor Canvas Component**

The construction of a syntax-aware text editor within the Fyne framework necessitates moving beyond standard UI components. The complexity of dynamic, token-level coloring and efficient handling of large files requires the development of a specialized custom widget, referred to here as the NONMEMEditor.

### **I.A. Rationale for Custom Widget Implementation**

Standard Fyne widgets, such as widget.Entry or widget.MultiLineEntry, are designed for simpler data capture and lack the granular control required for true syntax highlighting. While widget.RichText exists and can render text segments with basic styling, it typically converts the input into a single segment or predefined segment types, making it unsuitable for the real-time, dynamic output of a lexical analyzer.1 For an editor component that must manage cursor positioning, handle key events, and apply distinct colors and styles to individual lexical tokens, a custom widget is mandatory.

The custom widget paradigm in Fyne involves defining the component's state and behavior (fyne.Widget) separately from its visual representation (fyne.WidgetRenderer).2 This separation promotes clean testing and architectural clarity.3 The NONMEMEditor will act as the state manager, handling user inputs and coordinating the lexical analysis, while the associated nonmemRenderer will manage the efficient drawing of primitives.

### **I.B. Definition and State Management of NONMEMEditor**

The internal state of the editor is defined by the NONMEMEditor struct, which embeds widget.BaseWidget to acquire essential canvas object functionalities.2

#### **Data Model Design**

To facilitate high-speed editing operations (insertion and deletion), the text content is managed internally as a slice of runes (TextContentrune).4 Golang's strings are immutable, meaning that simple operations like character insertion require creating and copying data into a new string, leading to performance degradation in large files. By using a rune slice, text modification can be accomplished using efficient slice operations, ensuring minimal latency during typing.

The results of the lexical analysis are stored in a critical data structure: the LinesLineData array. This structure serves as a pre-computed cache for the renderer, ensuring the expensive task of tokenizing is decoupled from the rapid drawing cycles. Each LineData object stores the line's content, the array of identified Token structures, and pre-calculated vertical offsets, essential for virtualization.

Table I defines the essential Go structures underpinning the custom component.

Table I: Core Go Structures for the NONMEMEditor Component

| Struct Name | Purpose | Key Fields/Type |
| :---- | :---- | :---- |
| Token | Represents a single lexed segment (lexeme). | Type TokenType, Value string, StartPos int, EndPos int |
| LineData | Stores tokens and visual metrics for a single source line. | Contentrune, TokensToken, YOffset float32, LineHeight float32 |
| NONMEMEditor | The custom Fyne widget (state and behavior). | widget.BaseWidget, TextContentrune, LinesLineData, CursorPos int, Viewport fyne.Size |
| nonmemRenderer | The rendering backend (visual elements). | LineObjects\*canvas.Text, Cursor \*canvas.Rectangle, Parent \*NONMEMEditor, ScrollOffset fyne.Position |

#### **Content and Cursor State**

The NONMEMEditor must maintain the logical cursor position (CursorPos int), which is an index into the TextContentrune slice. This single index simplifies text manipulation but requires sophisticated mapping within the renderer to translate the linear index back to a two-dimensional (line, column) coordinate for visual display. The editor's primary role is to manage these state variables, trigger the Lexer when TextContent changes, and call the renderer's Refresh() method when the results are ready.

### **I.C. Input and Interaction Handling**

To process user input, the NONMEMEditor must implement the fyne.Focusable interface, which provides hooks for capturing focus events and keyboard input.5

#### **Key Event Processing (TypedKey)**

Structural inputs, such as movement keys (arrows, Home, End), deletion keys (Backspace, Delete), and line breaks (Enter), are handled by the TypedKey function.6

1. **Cursor Movement:** Arrow keys manipulate the CursorPos, adjusting it to the start or end of lines as needed.  
2. **Structural Modification:** The Enter key inserts the newline rune (\\n) into the TextContent slice. This operation must also calculate and insert the appropriate indentation based on the preceding line, common behavior for code editors. The modification of TextContent immediately necessitates a new lexical pass.

#### **Text Event Processing (TypedRune)**

The TypedRune(r rune) function captures character input, such as letters, numbers, and symbols. Upon receiving a rune, the editor inserts the character into the TextContentrune at CursorPos, increments CursorPos, and then initiates the asynchronous lexical analysis pipeline to update the highlighting.6

### **I.D. The nonmemRenderer Architecture and Virtualization**

The nonmemRenderer implements the fyne.WidgetRenderer interface, managing the visual representation of the editor state.2 Its primary functions are MinSize(), Layout(), and Refresh().

#### **Layout and Sizing (MinSize, Layout)**

1. **MinSize() Calculation:** The renderer must calculate the total size required for the entire document, regardless of the viewport size. This is crucial for correctly sizing the scrollbars of the surrounding scroll container (e.g., container.Scroll).7 The minimum height is the sum of all line heights (LineHeight), and the minimum width is the width of the longest tokenized line. This assumes a fixed-width (monospaced) font, where character width is consistent, allowing easy calculation of pixel width based on character count.8  
2. **Layout() Implementation:** The Layout(size fyne.Size) function positions the visible canvas objects within the allocated space.9 Since the editor employs virtualization, this function only concerns itself with positioning the small subset of canvas.Text objects that are currently visible within the scrolling viewport.

#### **Performance Optimization through Text Virtualization**

Handling large NONMEM control files efficiently requires virtualization. Rendering thousands of graphical primitives, even simple text objects, leads to significant performance drain, especially during rapid scrolling and repeated refresh cycles, which can cause high CPU utilization.10

The implementation relies on line indexing and clipping the content to the viewport.

1. **Line Indexing and Visibility:** The renderer calculates the vertical scroll offset provided by the containing scroll component. Using the pre-calculated YOffset metrics in LineData, the renderer determines the index of the first visible line ($L\_{start}$) and the last visible line ($L\_{end}$). This determination of a "coarse" position based on accumulated line size is foundational to high-performance text rendering.12  
2. **Recycling Canvas Primitives:** Instead of destroying and recreating the canvas.Text objects for every line that scrolls out and a new line scrolls in, the renderer maintains a pool of reusable primitives (LineObjects). When a line scrolls out, its associated canvas.Text objects are retained. When a new line scrolls into view, the renderer updates the existing primitive's Text, Color, and Position based on the new visible line's tokens. This recycling significantly reduces the overhead associated with GPU data transfer and object instantiation, ensuring smooth scrolling performance.11

## **II. The NONMEM Lexical Analyzer (Lexer) Design**

To implement syntax highlighting, a specialized Lexer capable of understanding the NONMEM control stream grammar is required. This Lexer will be built using the Go functional state machine pattern, widely accepted for its robustness and clarity in tokenizing languages.14

### **II.A. Adopting the Go Functional State Machine Pattern**

The Lexer operates as a state machine, traversing the input text (TextContentrune) and classifying segments (lexemes) into specific tokens.14 The core mechanism involves a Lexer struct that manages the input buffer and position, and uses stateFn functions to define transitions.15

Go

type Lexer struct {  
    inputrune  
    start int  
    pos   int  
    tokens chan Token // Output channel for tokens  
}  
type stateFn func(\*Lexer) stateFn // The functional state transition

This functional approach ensures that token recognition is context-aware. For instance, whether an identifier is a standard variable or a specific NONMEM keyword is determined by the state functions that manage the reading process.16 The Lexer emits recognized tokens asynchronously into a channel, which is then consumed by the editor's main logic for structuring into LineData.

### **II.B. Comprehensive NONMEM Grammar Specification**

Effective highlighting requires precise classification of the specialized vocabulary used in pharmacometric modeling. NONMEM control streams are characterized by records starting with the dollar sign ($).17

#### **Directive and Record Structure**

Primary directives, which define the structure and scope of the run, must be recognized first. These include $PROBLEM, $DATA, $ESTIMATION, and $TABLE.17  
Section keywords delineate modeling blocks, such as $PK (Pharmacokinetics), $ERROR (Residual variability model), $OMEGA (Inter-individual variability, $\\eta$), and \`$SIGMA$ (Residual variability, $\\epsilon$).19

#### **Pharmacometric and Data Item Keywords**

The Lexer must also recognize standard data item identifiers used in $INPUT and $TABLE records. These include column labels such as ID, TIME, DV (Dependent Variable), AMT (Amount), EVID (Event ID), and MDV (Missing Dependent Variable).18

Furthermore, modeling-specific keywords must be captured, such as estimation methods like FOCE (First Order Conditional Estimation), FOI (First Order with Interaction), and structural keywords like PREDPP and MAXEVALS.19

Table II specifies the relationship between the language components and their required token classifications for highlighting.

Table II: NONMEM Control Stream Lexical Categories and Token Map

| Lexical Category | Example Syntax | Assigned Token Type | Role/Context |
| :---- | :---- | :---- | :---- |
| Primary Directives | $PROBLEM, $DATA, $ESTIMATION, $TABLE | TokenDirective | Top-level command records 17 |
| Section Keywords | $PK, $PRED, $ERROR, $OMEGA, $SIGMA | TokenSectionKeyword | Defines specific modeling blocks 19 |
| Data Keywords | ID, TIME, DV, AMT, EVID, MDV, NOPRINT | TokenDataKeyword | Data file column labels and output controls 18 |
| Model Keywords | ADVAN1, FOCE, FOI, PREDPP, MAXEVALS | TokenModelKeyword | Estimation or prediction method keywords 19 |
| Parameter Identifiers | THETA, ETA, EPSILON, CL, V, KA | TokenParameterID | Pharmacometric variables 20 |
| Comments | ; description, ;; Based on: 001.mod | TokenComment | Everything following a semi-colon to EOL 18 |
| Numeric Literals | 0.1, 1E-3, (3, 5, 11\) | TokenLiteralNumber | Parameter estimates or data values 18 |

### **II.C. Detailed State Function Implementation**

The lexing process starts with lexStart, which typically consumes whitespace and looks for the start of tokens.

#### **The Complex lexComment State**

The most nuanced part of the NONMEM Lexer is handling comments. In the control stream, the semicolon (;) initiates a comment that runs until the end of the line.18 This character can appear anywhere, including immediately after parameter declarations, providing inline descriptions, for example: $THETA (3, 5, 11\) ; CL/F (10, 50, 100\) ; V/F.18

This structural element requires that every state function responsible for reading non-comment elements (e.g., lexNumericLiteral, lexIdentifier) must check immediately for the semicolon. If a semicolon is encountered, the Lexer must cease its current parsing function and transition immediately to the lexComment state. The lexComment function consumes all subsequent runes until a newline (\\n) is reached, tokenizing the entire sequence as a single TokenComment. This prioritization ensures that descriptive text, which might otherwise contain recognized keywords or identifiers (e.g., the CL/F notation), is correctly colored as commentary, maintaining the integrity of the highlighting scheme.

#### **lexNumericLiteral**

This state function is responsible for recognizing continuous sequences of digits, decimal points, and scientific notation indicators. It must also accommodate the specific structure used for parameter estimation bounds, where values are often enclosed in parentheses and separated by commas, such as in $THETA (3, 5, 11).18 The state function consumes the numbers and separators, emitting a single TokenLiteralNumber or an array of them depending on implementation granularity.

## **III. Integration and Highlighting Pipeline Implementation**

The final stage involves bridging the asynchronous Lexer output to the Fyne renderer, ensuring thread safety and visual accuracy.

### **III.A. Synchronization and Asynchronous Processing**

The core processing methodology involves concurrent execution. The Lexer must run in a background goroutine, processing the potentially large TextContent buffer without freezing the main GUI thread.

#### **The fyne.Do Concurrency Wrapper**

Modern Fyne toolkits (v2.6.0 onwards) enforce a single-goroutine model for all UI callbacks, events, and rendering to eliminate data races and guarantee smooth animations.23 This design constraint means that the background Lexer goroutine is prohibited from directly modifying the NONMEMEditor's state (e.g., updating editor.Lines) or calling editor.Refresh().

To safely synchronize the lexed output with the UI, the Lexer must send its completed LineData back to the main widget logic via a channel. The code receiving this update must wrap all subsequent Fyne UI calls in fyne.Do or fyne.DoAndWait.23 For example:

Go

// Inside the Lexer's completion routine (running in background goroutine):  
//... calculate newLines  
fyne.Do(func() {  
    editor.Lines \= newLines  
    editor.Refresh()  
})

This step is critical for robustness. By explicitly using fyne.Do, the application marshals the state update onto Fyne’s dedicated rendering goroutine, preventing concurrency conflicts that would otherwise lead to application instability or visual tearing.

### **III.B. Rendering Segmented Text Primitives**

Fyne's rendering model dictates that the visual attributes of a text string—color, size, and style—are uniform across the entire primitive.8 Therefore, to display syntax highlighting where different words (tokens) have different colors on the same line, the renderer must generate a sequence of individual canvas.Text objects, one for each token.

#### **Token-to-Style Mapping (Theme-Aware)**

A fundamental requirement for a modern graphical application is seamless integration with user themes (e.g., light mode vs. dark mode). The TokenStyleMap translates the TokenType (e.g., TokenDirective) into specific Fyne styling parameters.

The colors utilized in this map must not be hardcoded RGB values. Instead, they must dynamically query the current active Fyne theme using functions like theme.DefaultTheme().Color(name, variant).25 Since the nonmemRenderer.Refresh() method is explicitly triggered when the underlying theme is altered 2, re-querying the theme colors during refresh ensures that the highlighting scheme automatically adapts to the user's chosen theme (e.g., switching from bright blue text on a light background to a muted cyan text on a dark background).26

Table III illustrates the required theme-aware mapping.

Table III: Token-to-Fyne Style Configuration

| Token Type | Role | Fyne Theme Color Reference | Fyne TextStyle |
| :---- | :---- | :---- | :---- |
| TokenDirective | Primary Commands ($PROBLEM) | theme.ColorNamePrimary | fyne.TextStyle{Bold: true} |
| TokenSectionKeyword | PK/PD Blocks ($PK, $OMEGA) | theme.ColorNameAccent | fyne.TextStyle{Bold: true} |
| TokenDataKeyword | Data Col. Headers (MDV, TIME) | theme.ColorNameForeground | fyne.TextStyle{Italic: true} |
| TokenParameterID | Pharmacometric Vars (CL, V, ETA) | Custom Color (Themed Palette) | fyne.TextStyle{} |
| TokenLiteralNumber | Numeric Values | theme.ColorNameForeground | fyne.TextStyle{} |
| TokenComment | Semicolon Comments | theme.ColorNameDisabled | fyne.TextStyle{Italic: true} |
| TokenError | Lexing or Parsing Failure | theme.ColorNameError | fyne.TextStyle{Underline: true} |

### **III.C. Line Layout and Cursor Placement**

The nonmemRenderer must precisely arrange the sequence of token primitives horizontally and manage the blinking cursor vertically.

#### **Horizontal Layout Calculation**

The renderer iterates through the tokens of a visible line. It maintains a running horizontal offset, $X\_{pos}$. The first token is placed at $X\_{pos}$. The subsequent token must be placed immediately adjacent. The width of each canvas.Text object is determined by calling its MinSize() method. Since a fixed-width font is mandated for code editing, this size corresponds accurately to the pixel width of the token's text string.8

The layout cycle for a line:

1. Initialize $X\_{pos}$ to the left edge of the content area.  
2. For Token $i$:  
   a. Place $Token\_{i}$ at position $(X\_{pos}, LineData.YOffset)$.  
   b. Calculate the width of $Token\_{i}$: $W\_{i} \= Token\_{i}.MinSize().Width$.  
   c. Update $X\_{pos} \\leftarrow X\_{pos} \+ W\_{i}$.

This cumulative placement ensures seamless visual rendering across token boundaries.

#### **Cursor Rendering**

The cursor is rendered as a simple, thin canvas.Rectangle primitive. Its position is derived by mapping the logical CursorPos index from the TextContent array back to its physical screen coordinates $(X, Y)$.

Given the fixed-width font, the character width, $W\_{char}$, is constant. If the cursor is at column $C$ on line $L$, the $X$ coordinate is calculated as $X \= ScrollOffset.X \+ C \\times W\_{char}$. The $Y$ coordinate is directly sourced from $Line\_{L}.YOffset$. This fixed geometry ensures the cursor primitive snaps accurately to the character grid, a prerequisite for any functional code editor. The visibility of the cursor must be toggled on FocusGained() and FocusLost() events.5

#### **Line Number Panel**

For professional code viewing, a line number column is essential. This is architected as a separate, thin panel situated to the left of the main code content. It utilizes its own set of recycled canvas.Text primitives, rendering only the numbers corresponding to the visible lines ($L\_{start}$ to $L\_{end}$). This column is positioned using a fyne.Container.NewHBox() arrangement that ensures vertical scrolling is synchronized with the main editor content while maintaining static horizontal placement.

## **IV. Conclusion and Future Development Roadmap**

### **IV.A. Summary of Architectural Achievements**

The proposed implementation plan delivers a high-performance, domain-specific text editor component for Fyne based on robust architectural patterns.

1. **Custom Control:** The use of a custom NONMEMEditor widget and nonmemRenderer provides the necessary low-level control for token-based coloring, impossible with standard Fyne components.  
2. **Lexical Fidelity:** Implementation of the Golang functional state machine Lexer provides precise and context-aware recognition of complex NONMEM control stream syntax, including primary directives, parameter descriptors, and inline comments.  
3. **Performance Guarantee:** The design incorporates aggressive performance optimizations, specifically text virtualization and primitive recycling, ensuring the editor remains fluid and responsive even when loading and scrolling through large pharmacometric control files.  
4. **Platform Robustness:** Strict adherence to Fyne’s thread safety requirement through the use of fyne.Do ensures that the asynchronous lexical analysis pipeline does not introduce data race conditions or UI corruption, aligning the system with the toolkit's current evolution. Furthermore, dynamic color lookups guarantee theme-aware display across user settings.

### **IV.B. Roadmap for Production Features**

To evolve the NONMEMEditor into a production-ready tool for pharmacometric engineers, the following features are recommended for future development:

1. **Basic Text Editing Operations and History:** Implementation of a robust Undo/Redo history mechanism is paramount. This can be achieved by utilizing a command pattern that records changes (diffs) to the TextContent buffer and tracks corresponding CursorPos states, providing the user with reliable recovery from modifications.27  
2. **Rudimentary Parser Integration for Semantic Feedback:** While the current system only performs lexical analysis, integrating a basic parser (which consumes the token stream) would allow for semantic validation. This could identify structural errors (e.g., a missing required keyword or improperly nested blocks) and utilize the TokenError type to visually mark the offending syntax elements with error coloring and potentially an underline.10  
3. **Code Folding:** For managing the visual complexity of large control streams, implementing code folding based on structural directives (e.g., collapsing the body content beneath $PK, $ERROR, or $THETA records) would greatly enhance navigability and user experience.

#### **Works cited**

1. widget.RichText \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/api/v2.2/widget/richtext.html](https://docs.fyne.io/api/v2.2/widget/richtext.html)  
2. Writing a Custom Widget | Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/extend/custom-widget/](https://docs.fyne.io/extend/custom-widget/)  
3. Widgets \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/architecture/widgets/](https://docs.fyne.io/architecture/widgets/)  
4. Building a collaborative text editor in Go \- Aadhav Vignesh, accessed November 8, 2025, [https://databases.systems/posts/collaborative-editor](https://databases.systems/posts/collaborative-editor)  
5. fyne.Focusable \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/api/v2.4/focusable.html](https://docs.fyne.io/api/v2.4/focusable.html)  
6. widget package \- fyne.io/fyne/v2/widget \- Go Packages, accessed November 8, 2025, [https://pkg.go.dev/fyne.io/fyne/v2/widget](https://pkg.go.dev/fyne.io/fyne/v2/widget)  
7. fyne.Scrollable \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/api/v2/fyne/scrollable/](https://docs.fyne.io/api/v2/fyne/scrollable/)  
8. Fyne API “canvas.Text” \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/api/v1.4/canvas/text.html](https://docs.fyne.io/api/v1.4/canvas/text.html)  
9. Building a Custom Layout | Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/extend/custom-layout/](https://docs.fyne.io/extend/custom-layout/)  
10. efficient canvas refresh with fyne golang \- Stack Overflow, accessed November 8, 2025, [https://stackoverflow.com/questions/67359902/efficient-canvas-refresh-with-fyne-golang](https://stackoverflow.com/questions/67359902/efficient-canvas-refresh-with-fyne-golang)  
11. performance \- Im having trouble with Fyne in GoLang \- Stack Overflow, accessed November 8, 2025, [https://stackoverflow.com/questions/78804835/im-having-trouble-with-fyne-in-golang](https://stackoverflow.com/questions/78804835/im-having-trouble-with-fyne-in-golang)  
12. Algorithm for Rendering Long Text in a Text Editor \- Stack Overflow, accessed November 8, 2025, [https://stackoverflow.com/questions/4443800/algorithm-for-rendering-long-text-in-a-text-editor](https://stackoverflow.com/questions/4443800/algorithm-for-rendering-long-text-in-a-text-editor)  
13. Scrolling is really slow · Issue \#516 · fyne-io/fyne \- GitHub, accessed November 8, 2025, [https://github.com/fyne-io/fyne/issues/516](https://github.com/fyne-io/fyne/issues/516)  
14. zalgonoise/lex: a generic lexer library written in Go \- GitHub, accessed November 8, 2025, [https://github.com/zalgonoise/lex](https://github.com/zalgonoise/lex)  
15. Implementing Functional State Machines with Go \- Sophora CMS by subshell, accessed November 8, 2025, [https://subshell.com/blog/go-functional-state-machines100.html](https://subshell.com/blog/go-functional-state-machines100.html)  
16. Writing a Lexer in Go with LexMachine \- Hackthology, accessed November 8, 2025, [https://hackthology.com/writing-a-lexer-in-go-with-lexmachine.html](https://hackthology.com/writing-a-lexer-in-go-with-lexmachine.html)  
17. NONMEM Tutorial Part I: Description of Commands and Options, With Simple Examples of Population Analysis \- PubMed Central, accessed November 8, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC6709426/](https://pmc.ncbi.nlm.nih.gov/articles/PMC6709426/)  
18. NONMEM template control file syntax \- Certara, accessed November 8, 2025, [https://onlinehelp.certara.com/pirana/21.11.1/Pirana\_User\_Guide/Pirana\_NONMEM/NONMEM\_template\_control\_file\_syntax.htm](https://onlinehelp.certara.com/pirana/21.11.1/Pirana_User_Guide/Pirana_NONMEM/NONMEM_template_control_file_syntax.htm)  
19. Standard Error of Empirical Bayes Estimate in NONMEM® VI \- PMC \- PubMed Central, accessed November 8, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC3339294/](https://pmc.ncbi.nlm.nih.gov/articles/PMC3339294/)  
20. (PDF) Tips for the choice of initial estimates in NONMEM \- ResearchGate, accessed November 8, 2025, [https://www.researchgate.net/publication/308387723\_Tips\_for\_the\_choice\_of\_initial\_estimates\_in\_NONMEM](https://www.researchgate.net/publication/308387723_Tips_for_the_choice_of_initial_estimates_in_NONMEM)  
21. Pharmacometric models simulation using NONMEM, Berkeley Madonna and R \- PMC, accessed November 8, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC7033377/](https://pmc.ncbi.nlm.nih.gov/articles/PMC7033377/)  
22. III. Control Records \- NONMEM Help, accessed November 8, 2025, [https://nmhelp.tingjieguo.com/IV/III](https://nmhelp.tingjieguo.com/IV/III)  
23. Fyne v2.6 alpha1 \- Fyne.io, accessed November 8, 2025, [https://fyne.io/blog/2025/02/11/2.6-alpha1.html](https://fyne.io/blog/2025/02/11/2.6-alpha1.html)  
24. Fyne API “canvas.Text” \- Fyne documentation, accessed November 8, 2025, [https://docs.fyne.io/api/v2.4/canvas/text.html](https://docs.fyne.io/api/v2.4/canvas/text.html)  
25. Creating a Custom Theme \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/extend/custom-theme/](https://docs.fyne.io/extend/custom-theme/)  
26. Theme and Customisation \- Fyne Documentation, accessed November 8, 2025, [https://docs.fyne.io/faq/theme/](https://docs.fyne.io/faq/theme/)  
27. Lucifer25x/fyne-text-editor: Simple Text Editor using Fyne (go) \- GitHub, accessed November 8, 2025, [https://github.com/Lucifer25x/fyne-text-editor](https://github.com/Lucifer25x/fyne-text-editor)

---

## **V. Implementation Feasibility Analysis and Practical Considerations**

### **V.A. Tenability Assessment: HIGH ✅**

The proposed architecture is **sound and implementable**. The plan demonstrates excellent understanding of both Fyne's internal architecture and the fundamental requirements for building a high-performance text editor. The approach is idiomatic to Go and follows established patterns from production text editors.

### **V.B. Critical Success Factors**

The plan correctly identifies and addresses the major technical challenges:

1. **Custom Widget Architecture** - The decision to build `NONMEMEditor` as a custom widget rather than attempting to extend `widget.Entry` or hack `widget.RichText` is fundamentally correct. Standard Fyne widgets lack the granular control required for token-level coloring.

2. **Performance Strategy** - The three-pronged performance approach is exactly what production editors use:
   - Rune slice storage for O(1) character insertion/deletion
   - Text virtualization to render only visible lines
   - Canvas primitive recycling to prevent GPU memory thrashing

3. **Thread Safety** - Recognition of Fyne v2.6+ single-goroutine requirement and proper use of `fyne.Do()` is critical. Without this, asynchronous lexing would cause data races and UI corruption.

4. **Lexer Design** - The functional state machine pattern is the Go community standard for lexers (used by Go's own `text/template` parser). The asynchronous token emission via channels correctly decouples expensive analysis from rendering.

### **V.C. Implementation Challenges and Mitigation Strategies**

While the architecture is sound, several implementation complexities require careful handling:

#### **Challenge 1: Cursor Position Mapping**

**Problem**: The plan stores cursor position as a linear index into the `[]rune` buffer, but rendering requires 2D `(line, column)` coordinates. Mapping between these representations is non-trivial, especially with variable-length lines and multi-byte Unicode characters.

**Solution**:
- Maintain a line break index (`[]int`) during lexing that stores the `TextContent` index of each `\n` character
- To find line number from `CursorPos`: Binary search the line break index
- To find column: Subtract line start index from `CursorPos`
- Must handle rune boundaries correctly - Go strings are UTF-8, so byte offsets ≠ rune offsets

```go
// Example line break index structure
type LineIndex struct {
    breakPoints []int // Indices in TextContent where \n occurs
}

func (li *LineIndex) GetLineCol(pos int) (line, col int) {
    line = sort.SearchInts(li.breakPoints, pos)
    if line == 0 {
        col = pos
    } else {
        col = pos - li.breakPoints[line-1] - 1
    }
    return
}
```

#### **Challenge 2: Text Selection Support**

**Problem**: The plan does not explicitly address text selection (highlight-dragging with mouse), which is essential for copy/paste operations.

**Solution**:
- Add `SelectionStart` and `SelectionEnd` fields to `NONMEMEditor`
- Implement `fyne.Draggable` interface to capture mouse drag events
- Render selection as a series of `canvas.Rectangle` primitives behind the text
- Selection rectangles must be virtualized along with text to maintain performance
- Handle multi-line selections by calculating start/end coordinates for each visible line

#### **Challenge 3: Horizontal Scrolling**

**Problem**: While the plan thoroughly addresses vertical virtualization, NONMEM control files often contain very long lines (e.g., `$TABLE` records with many columns). Without horizontal scrolling, content will be clipped.

**Solution**:
- Track `MaxLineWidth` during lexing (width of longest line in pixels)
- Use `container.NewScroll()` with both `ScrollHorizontally` and `ScrollVertically` enabled
- The renderer's `MinSize()` must return `fyne.NewSize(MaxLineWidth, TotalHeight)`
- Horizontal scroll offset must be factored into token `X` position calculations

#### **Challenge 4: Input Method Editor (IME) Support**

**Problem**: Complex character input (accented characters via compose keys, Asian language IME) requires special handling beyond simple `TypedRune()` events.

**Mitigation Options**:
1. **Full Support**: Implement `fyne.TextInputHandler` interface to receive pre-edit and commit events (significant complexity)
2. **Partial Support**: Accept limitations - IME input may be clunky but functional via basic events
3. **Defer to Fyne**: For MVP, rely on Fyne's built-in keyboard handling and accept potential IME issues

**Recommendation**: Start with option 2 (partial support) for MVP, upgrade to option 1 only if users report issues.

#### **Challenge 5: Undo/Redo System**

**Problem**: Mentioned in roadmap but critical for any production editor. Naive implementations (storing full text snapshots) consume excessive memory for large files.

**Solution - Command Pattern with Diffs**:
```go
type EditCommand interface {
    Execute(*NONMEMEditor) error
    Undo(*NONMEMEditor) error
}

type InsertCommand struct {
    pos     int
    content []rune
}

type DeleteCommand struct {
    pos     int
    content []rune // Store deleted content for undo
}

type UndoStack struct {
    commands []EditCommand
    position int // Current position in stack
}
```

- Each edit operation creates a command object
- Command stores only the diff (position + changed content), not full text
- Stack typically limited to last N commands (e.g., 100-1000) to bound memory
- Redo is implemented by moving forward in the stack

### **V.D. NONMEM Grammar Coverage Validation**

The token categorization (Table II) is comprehensive, but implementation should verify:

1. **Abbreviated Record Names**: NONMEM accepts both full and abbreviated forms:
   - `$ESTIMATION` vs `$EST`
   - `$OMEGA` vs `$OME` vs `$OMEG`
   - Lexer must recognize all variants

2. **Nested Control Structures**: `$PK` and `$ERROR` blocks can contain IF/THEN/ELSE logic:
   ```
   $PK
   IF (AMT.GT.0) THEN
     F1 = THETA(1) * EXP(ETA(1))
   ENDIF
   ```
   - Decision needed: Highlight FORTRAN keywords (IF, THEN, ELSE, ENDIF) or treat as generic identifiers?

3. **Version-Specific Keywords**: Different NONMEM versions (7.3, 7.4, 7.5) have introduced new keywords:
   - NONMEM 7.4+: `NUMERICAL` derivatives, `LIKELIHOOD` methods
   - Lexer should be version-aware or recognize superset of all versions

4. **Comments Containing Code**: Pharmacometricians often comment out blocks by prefixing each line with `;`. Lexer must not attempt to lex commented code:
   ```
   ;$THETA (0, 5, 100) ; CL - THIS IS ALL COMMENT
   ```

### **V.E. Implementation Phases and Effort Estimation**

#### **Phase 1: Minimal Viable Editor (2-3 weeks)**
- Custom widget shell (`NONMEMEditor`, `nonmemRenderer`)
- Basic text rendering (no highlighting, monochrome)
- Cursor positioning and keyboard navigation
- Simple text input (no selection, no undo)
- Vertical scrolling with virtualization
- **Deliverable**: Functional plain text editor in Fyne

#### **Phase 2: Syntax Highlighting (2-3 weeks)**
- NONMEM lexer implementation (functional state machine)
- Token categorization for core keywords (Table II)
- Async lexing pipeline with `fyne.Do()` synchronization
- Theme-aware color mapping (Table III)
- Token rendering with individual `canvas.Text` primitives
- **Deliverable**: Read-only syntax-highlighted NONMEM viewer

#### **Phase 3: Full Editing Capabilities (1-2 weeks)**
- Text selection with mouse (drag events)
- Copy/paste integration with system clipboard
- Horizontal scrolling for long lines
- Search/replace functionality (optional but highly valuable)
- **Deliverable**: Feature-complete editor (minus undo/redo)

#### **Phase 4: Production Polish (1-2 weeks)**
- Undo/redo command stack
- Line number panel (virtualized)
- Performance profiling and optimization
- Edge case handling (empty files, binary files, huge lines)
- **Deliverable**: Production-ready NONMEM editor

**Total Effort Estimate**: 6-10 weeks for single developer working full-time

### **V.F. Alternative Approaches and Trade-Offs**

#### **Alternative 1: Adapt Existing Syntax Highlighter Library**

**Option**: Use `github.com/alecthomas/chroma` (generic syntax highlighter supporting 200+ languages)

**Pros**:
- Chroma has lexers for many languages, could add NONMEM lexer definition (XML-based)
- Well-tested tokenization logic

**Cons**:
- Chroma lexers are regexp-based, not functional state machines (less flexible for NONMEM's quirks)
- **Still requires full custom Fyne widget** - cannot integrate with standard `widget.Entry` or `widget.RichText`
- The rendering pipeline (token-level `canvas.Text` primitives, virtualization, cursor management) is identical whether using Chroma or custom lexer
- Less control over token granularity

**Prior Experience**: Previously attempted Chroma integration. Result: **Same implementation effort required** because Fyne's standard widgets cannot render token-level highlighting. The lexer choice (Chroma vs custom) only affects ~15-20% of total implementation work.

**Verdict**: **Not recommended**. The effort savings are minimal (~1 week) while sacrificing NONMEM-specific intelligence and lexer flexibility. Full custom implementation is warranted.

#### **Alternative 2: Read-Only Viewer First**

**Option**: Implement syntax highlighting without editing capabilities initially

**Pros**:
- Eliminates cursor management, text selection, undo/redo complexity
- Reduces Phase 1 + Phase 2 effort to ~3-4 weeks

**Cons**:
- Users must edit in external editor, reducing Janus workflow integration
- **Zero practical value** - syntax highlighting without editing capability provides no benefit to users who need to modify models

**Verdict**: **Not recommended**. Syntax highlighting is only valuable when combined with editing capability. A read-only viewer would be a wasted implementation effort.

#### **Alternative 3: Hybrid Approach - Basic Editor + External Tools**

**Option**: Implement basic text editing in standard `widget.Entry`, add "Open in VS Code" button for power users

**Pros**:
- Minimal implementation effort (days, not weeks)
- Leverages best-in-class external editors (VS Code, Sublime) for complex editing
- VS Code has NONMEM syntax extension available

**Cons**:
- Breaks workflow - forces context switch to external tool
- Requires users to install/configure VS Code

**Verdict**: Acceptable fallback if full custom editor effort cannot be justified.

### **V.G. Strategic Recommendation and Priority Assessment**

**Context**: Janus currently has higher-priority issues:
- MSI installer launcher failures (critical user-facing bug)
- Hermes executor feature completion (core functionality)
- SLURM integration and grid monitoring (differentiating feature)

**Recommendation**:

1. **Short-term (next 2 months)**: Use standard `widget.Entry` for model editing. Focus on fixing launcher issues and completing Hermes integration.

2. **Medium-term decision point**: Either commit to **full custom editor implementation** (all 4 phases, 6-10 weeks) OR use **Alternative 3** (basic editor + "Open in External Editor" button).

3. **Long-term (if full editor selected)**: Phased rollout:
   - Months 3-4: Phase 1+2 (basic editor + syntax highlighting)
   - Month 5: Phase 3 (selection, copy/paste, horizontal scroll)
   - Month 6: Phase 4 (undo/redo, line numbers, polish)

**ROI Justification Questions**:
- Do target users edit models frequently within Janus, or primarily via external IDEs?
- Is syntax highlighting a "nice-to-have" or "critical differentiator" vs Pirana?
- Can the 6-10 week implementation effort be justified given current higher-priority features?
- Would Alternative 3 (external editor integration) satisfy 80%+ of use cases at 10% of the effort?

### **V.H. Risk Mitigation Checklist**

Before committing to full implementation:

- [ ] **Performance validation**: Prototype text virtualization with 10,000+ line dummy file, measure frame rate during scrolling
- [ ] **Fyne version compatibility**: Verify `fyne.Do()` behavior in current Fyne release (v2.5.x vs v2.6.x)
- [ ] **User research**: Survey 3-5 target users on editing workflow (edit in Janus vs external editor?)
- [ ] **Grammar completeness**: Obtain representative NONMEM control files from users, verify lexer handles all constructs
- [ ] **Cross-platform testing**: Verify rendering performance on Windows/Linux/macOS (OpenGL driver variations)

### **V.I. Success Metrics**

If proceeding with implementation, define measurable success criteria:

1. **Performance**:
   - Smooth scrolling (60 FPS) for files up to 5,000 lines
   - Lexing latency <100ms for typical control file (500 lines)
   - Memory usage <50MB for 10,000 line file

2. **Functionality**:
   - 95%+ token classification accuracy on validation corpus
   - Zero crashes during 1-hour continuous editing session
   - Undo/redo stack handles 100+ operations without memory issues

3. **User Satisfaction**:
   - User testing session: 4/5 users prefer Janus editor vs external editor for quick edits
   - Post-release survey: 80%+ users rate syntax highlighting as "useful" or "very useful"

---

**Document Status**: Feasibility analysis complete. Ready for implementation planning or alternative approach selection based on strategic priorities.