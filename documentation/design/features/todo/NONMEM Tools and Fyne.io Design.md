# **Architectural Analysis and Design Specification for Porting the Metrum Research Group Ecosystem (MERGE) to a Golang Fyne Application**

## **1\. Executive Overview of the Pharmacometric Toolchain Landscape**

The domain of pharmacometrics—specifically the application of nonlinear mixed-effects modeling (NLME) using NONMEM—demands a rigorous technological infrastructure to manage the complexities of model execution, data traceability, and results interpretation. In recent years, Metrum Research Group (MetrumRG) has established a definitive standard for this infrastructure through the Metrum Research Group Ecosystem, or MERGE. This ecosystem represents a shift from ad-hoc scripting to a disciplined, reproducible software development lifecycle applied to drug development analytics.1 For a systems architect or developer tasked with designing a next-generation interface using Golang and the Fyne.io toolkit, the challenge lies not merely in execution but in replicating the sophisticated logic that binds computation to analysis.  
The MERGE toolchain is architecturally bifurcated. The heavy lifting of execution, process management, and output parsing is handled by **bbi**, a high-performance command-line tool written in Go (Golang).3 Conversely, the nuanced logic of workflow management, model inheritance, and visualization is currently encapsulated in R packages such as **bbr**, **pmplots**, **pmtables**, and **nmrec**.4 This separation of concerns presents a unique engineering challenge for a purely Golang-based implementation: while the execution engine (bbi) is native and importable, the business logic for parameter inheritance and visualization must be ported from R to Go.  
This report provides an exhaustive analysis of the specific repositories and logic paths required to implement three core features: (1) Parameter Extraction, (2) Tabular and Graphical Display, and (3) Model Parameter Inheritance. By dissecting the internal mechanisms of the MERGE tools, this document lays the foundational architecture for a "Go-MERGE" Fyne application that creates a seamless, high-performance user experience for pharmacometricians.

## ---

**2\. The Metrum Research Group Ecosystem (MERGE): Architectural Decomposition**

To accurately target the correct tools for a Golang implementation, one must first understand the modular nature of MERGE. The ecosystem is designed as a "Bazaar" of open-source tools rather than a monolithic "Cathedral," allowing for individual components to be swapped or integrated independently.7 However, the interdependencies are strong, and understanding the flow of data between bbi (Go) and the R-layer is critical for replication.

### **2.1 The Execution Core: bbi (Go)**

The cornerstone of the ecosystem is **bbi** (Babylon Interface), located at github.com/metrumresearchgroup/bbi.3 This tool was explicitly designed to replace legacy Perl and Ruby scripts with a compiled, cross-platform binary that has zero external dependencies for the end-user.3  
In the context of the user's requirements, bbi is the primary asset. It is responsible for the actual submission of NONMEM jobs to the underlying compute infrastructure, whether that is a local machine, a Slurm cluster, or Sun Grid Engine (SGE).9 More importantly for the design of a GUI, bbi handles the parsing of NONMEM's notoriously complex and inconsistent output files (.lst, .ext, .cov) into structured JSON data.3  
For a Golang application, bbi is not just a reference; it is a library. The architectural strategy should involve importing the internal packages of bbi—specifically the runner and parsers packages—directly into the Fyne application. This allows the GUI to wrap the execution logic natively without the fragility of shelling out to a subprocess, although bbi is designed primarily as a CLI tool.3

### **2.2 The Workflow Manager: bbr (R)**

While bbi runs the models, **bbr** (github.com/metrumresearchgroup/bbr) manages them.4 This R package provides the "glue" that makes the raw execution useful to a scientist. It introduces concepts like "Model Objects" (S3 classes in R), "Tags," "Notes," and most critically for this report, the "Based On" ancestry tracking.10  
The logic for **Feature 3 (Inheritance)** resides almost entirely within bbr. When a user in R calls inherit\_param\_estimates(), bbr orchestrates the reading of the parent model's output and the rewriting of the child model's control stream.10 Because bbr is written in R, a Golang implementation cannot simply import this logic. Instead, the algorithms used by bbr must be reverse-engineered and re-implemented in Go. This involves replicating the file manipulation logic that bbr delegates to another package, nmrec.

### **2.3 The Control Stream Parser: nmrec (R)**

A hidden but vital component of the inheritance workflow is **nmrec** (github.com/metrumresearchgroup/nmrec).6 This package is a specialized parser for NONMEM control streams. It understands the idiosyncratic syntax of NONMEM records (e.g., $THETA, $OMEGA, $SIGMA) and allows for programmatic modification of these records.  
For the Golang implementation to support parameter inheritance, it essentially needs an embedded "mini-nmrec." The application must be able to read a text file, identify the specific lines corresponding to initial estimates, and replace them with numerical values extracted from a previous run, all while preserving comments and formatting. This is a non-trivial parsing task that is currently handled by R in the MERGE ecosystem.11

### **2.4 The Visualization Layer: pmplots and pmtables (R)**

The user's requirement for tabular display and plot details maps to **pmtables** and **pmplots** (github.com/metrumresearchgroup/pmtables, .../pmplots).5 These packages provide the specification for what constitutes a "standard" pharmacometric display. pmplots wraps the ggplot2 grammar of graphics to produce standardized diagnostic plots (e.g., DV vs. PRED, CWRES vs. TIME).5  
Since Fyne does not support R's plotting engines, the Golang implementation must recreate these visualizations using Go native libraries such as gonum/plot. The value of pmplots and pmtables to the designer is not their code, but their **specification**: they define exactly which columns from the NONMEM output tables need to be visualized and how they should be presented to the user.

## ---

**3\. Feature Analysis: Extracting Parameters from NONMEM Runs**

The first functional requirement is the ability to extract parameters from NONMEM runs. In the context of the MERGE ecosystem, this process is well-defined and heavily supported by the bbi tool's internal parsing logic.

### **3.1 The Role of bbi in Parameter Extraction**

The bbi tool was architected to solve the "parsing problem" of NONMEM. NONMEM output is text-heavy and formatted for human readability rather than machine consumption. bbi includes robust parsers that scrape these files and serialize the data into JSON.3  
The primary mechanism for this is the bbi nonmem summary command, specifically with the \--json flag. When executed, this command triggers the internal Go parsers to read the .lst (listing), .ext (estimation history), and .shk (shrinkage) files associated with a run.3

#### **3.1.1 Target Package: metrumresearchgroup/bbi/parsers**

For the design of the features, the developer should target the parsers package within the bbi repository. This package contains the Go structs and methods required to ingest NONMEM text files. Instead of writing regex parsers from scratch—which is error-prone due to the variability of NONMEM versions—the Fyne application should import this package.  
The parsers package likely exports structs similar to the JSON schema observed in the output 3:

Go

type RunDetails struct {  
    Theta           float64 \`json:"theta"\`  
    Omega           float64 \`json:"omega"\`  
    Sigma           float64 \`json:"sigma"\`  
    StdErr           struct {  
        Thetafloat64 \`json:"theta"\`  
        Omegafloat64 \`json:"omega"\`  
        Sigmafloat64 \`json:"sigma"\`  
    } \`json:"std\_err"\`  
    ShrinkageDetails struct {  
        Etafloat64 \`json:"eta\_shrinkage"\`  
        Epsfloat64 \`json:"eps\_shrinkage"\`  
    } \`json:"shrinkage\_details"\`  
}

The data extraction logic in the Fyne application becomes a function of mapping these internal bbi structs to the UI, rather than raw text parsing.

### **3.2 The Mathematical Context of Extraction**

To build a meaningful interface, the developer must understand what is being extracted. The "parameters" in a NONMEM run are divided into fixed effects (THETA) and random effects (OMEGA and SIGMA).

* **THETA**: These are the structural parameters of the model (e.g., Clearance, Volume of Distribution). They are vectors of scalar values.  
* **OMEGA**: This represents the inter-individual variability (IIV). In NONMEM, this is a variance-covariance matrix. The bbi parser typically extracts the upper or lower triangle of this matrix in a vectorized format.3 The Fyne application must understand how to reconstruct the matrix or display the diagonal elements (variances) and off-diagonal elements (covariances) appropriately.  
* **SIGMA**: This represents the residual unexplained variability (RUV) or intra-individual error. Like OMEGA, it is a matrix, though often diagonal.

The bbi summary output also includes **Standard Errors (SE)** for these estimates. A critical requirement for the UI is to calculate the **Relative Standard Error (RSE)** and **95% Confidence Intervals (CI)**, as these are rarely output directly by NONMEM but are standard for reporting.13 The logic for this calculation is found in pmparams but must be implemented in Go:

$$RSE(\\%) \= \\left| \\frac{SE}{Estimate} \\right| \\times 100$$

$$95\\% CI \= Estimate \\pm 1.96 \\times SE$$

### **3.3 Handling Shrinkage and Diagnostics**

The MERGE ecosystem places high value on diagnostic metrics like shrinkage. Shrinkage quantifies the extent to which individual parameter estimates (EBEs) "shrink" towards the population mean when the data is uninformative. High shrinkage (\>20-30%) invalidates certain diagnostic plots.14  
bbi parses shrinkage from the .shk file (if present) or the .lst file. The schema for shrinkage has evolved with NONMEM versions (e.g., NONMEM 7.4 introduced more shrinkage types). The bbi parser abstracts this complexity, providing a unified interface. The Fyne application should display these shrinkage values alongside the OMEGA estimates to provide context on the reliability of the random effects.

## ---

**4\. Feature Analysis: Tabular Display and Plot Details**

Once the data is extracted, the user requires a system for "Tabular display / plot details." This corresponds to the Reporting and Visualization layer of the MERGE ecosystem, dominated by pmtables and pmplots.

### **4.1 Tabular Display Architecture**

The goal of the tabular display in a Fyne application is to replace the static LaTeX tables generated by pmtables with an interactive grid. The data source for this table is the ParameterEstimate object constructed from the bbi parsers.

#### **4.1.1 Data Model for the Table Widget**

The Fyne widget.Table requires a structured data model. Based on the standard outputs defined in pmparams 15, the table should contain the following columns:

| Column Name | Data Source | Logic/Transformation |
| :---- | :---- | :---- |
| **Parameter** | bbi (Index) | THETA1, OMEGA(1,1), etc. |
| **Label** | Control Stream (.mod) | Parsed from comments (e.g., ; CL (L/h)). |
| **Estimate** | bbi (.ext/.lst) | Round to 3 sig figs (standard in pmparams). |
| **RSE (%)** | Derived | abs(SE/Est)\*100. |
| **95% CI** | Derived | Est \+/- 1.96\*SE. |
| **Shrinkage (%)** | bbi (.shk) | Only for OMEGA/SIGMA rows. |

#### **4.1.2 The Label Parsing Challenge**

A critical "unsatisfied requirement" in the bbi layer is the lack of parameter labels. bbi extracts the numerical values, but it does not parse the semantic meaning of the parameters. In the MERGE ecosystem, bbr handles this by reading the control stream file and extracting comments associated with parameter records.16  
To achieve parity with the MERGE toolchain, the Golang application must implement a **Control Stream Label Parser**. This parser needs to read the .mod file, identify lines starting with $THETA, $OMEGA, or $SIGMA, and capture the text following the comment delimiter (;). For example:

Code snippet

$THETA  
(0, 10\) ; Clearance (L/h)  
(0, 50\) ; Volume (L)

The Go parser must extract "Clearance (L/h)" and map it to THETA1. This logic is currently in bbr::param\_labels(), and porting it to Go is a prerequisite for a meaningful tabular display.

### **4.2 Plot Details and Visualization**

The "Plot details" requirement refers to the generation of diagnostic plots that allow the modeler to assess the goodness of fit. In pmplots, these are standardized ggplot2 objects.5

#### **4.2.1 Data Sources for Plots**

Unlike parameter tables, plots do not come from the .lst or .ext files. They are generated from the **Output Tables** produced by the $TABLE record in NONMEM. These files (often named sdtab, patab, cotab, catab) are whitespace-delimited ASCII files containing the raw data, predictions, and residuals.5  
bbi summarizes which table files were produced in its JSON output (output\_files\_used), but it does not parse the *content* of these large table files by default.3 The Fyne application must implement a high-performance reader (buffered I/O) to ingest these files, as they can be hundreds of megabytes in size for large population models.

#### **4.2.2 Standard Diagnostic Plots**

The application should target the following standard plots defined by the pmplots specification 17:

1. **DV vs. PRED (Observations vs. Population Predictions)**:  
   * **Logic**: Scatter plot of DV (y-axis) vs. PRED (x-axis).  
   * **Features**: Identity line ($y=x$), Log-Log toggle.  
2. **DV vs. IPRED (Observations vs. Individual Predictions)**:  
   * **Logic**: Scatter plot of DV vs. IPRED.  
   * **Significance**: Assesses how well the model fits individuals after accounting for random effects.  
3. **CWRES vs. TIME/PRED**:  
   * **Logic**: Scatter plot of Conditional Weighted Residuals vs. Time or Prediction.  
   * **Features**: Loess smoothing line (often red) and a zero-line. This detects bias (e.g., underprediction at late time points).  
4. **Distribution of Residuals**:  
   * **Logic**: Histogram or Q-Q plot of CWRES.  
   * **Significance**: Checks the assumption of normality in the error model.

#### **4.2.3 Implementing Plots in Fyne**

Fyne does not natively support scientific plotting. The recommended architecture is to use the **gonum/plot** library, which is the standard for scientific plotting in Go. The workflow would be:

1. Read the NONMEM table file into a struct Record.  
2. Create a plot.Plot object using gonum.  
3. Render the plot to a PNG buffer or a raster image.  
4. Display the image in a Fyne canvas.Image widget.

This approach maintains the "pure Go" requirement while leveraging a robust mathematical plotting library that is comparable in capability (though different in API) to ggplot2.

## ---

**5\. Feature Analysis: Model Inheritance (based\_on)**

The third requirement—the ability to *inherit* parameters—is the most complex from a software engineering perspective. It requires the application to actively modify the source code of the model, creating a lineage of analysis.

### **5.1 The based\_on Concept in MERGE**

In the MERGE ecosystem, provenance is tracked via the based\_on field in the model's YAML configuration.4 When a new model is created from an old one (e.g., run102 derived from run101), bbr records the parent's ID in 102.yaml. This establishes a directed acyclic graph (DAG) of model history.  
For the Golang implementation, the application must manage these YAML sidecar files (bbi.yaml or model.yaml). The application needs to use a YAML parser (like gopkg.in/yaml.v3) to read and update the based\_on array whenever a "Copy Model" or "Inherit" action is performed.

### **5.2 The Inheritance Algorithm (inherit\_param\_estimates)**

The core request is to "inherit parameters." This means taking the *final* estimates from the parent run and writing them as the *initial* estimates in the child run. This allows the child run to start searching from a more optimal point in the parameter space.

#### **5.2.1 The Gap: nmrec and Control Stream Modification**

In the R toolchain, bbr delegates this task to nmrec.6 nmrec parses the control stream into a programmable object, updates the values, and serializes it back to text. **There is no Go equivalent to nmrec in the public repositories.** The bbi parsers are read-only (output parsing).  
Therefore, the Golang application must implement a **Control Stream Patcher**. This component must perform the following algorithmic steps:

1. **Resolve Parent Path**: Look up the based\_on field in the child's YAML to find the parent model ID.  
2. **Load Parent Estimates**: Use the bbi output parser (Feature 1\) to load the final estimates (theta, omega, sigma) from the parent's .ext or .lst file.  
3. **Read Child Control Stream**: Load the .mod file of the child run into memory.  
4. **Locate Parameter Blocks**: Use a state-machine or regex approach to identify the boundaries of $THETA, $OMEGA, and $SIGMA blocks.  
5. **Record Matching and Replacement**:  
   * **$THETA**: Iterate through the records. NONMEM allows distinct formats: (INIT), (LOWER, INIT, UPPER), (LOWER, INIT, UPPER) FIXED. The parser must identify the INIT position and replace it with the parent's value.  
   * *Constraint*: The patcher must respect bounds. If the parent estimate is 0.5 but the child has bounds (0, 0.4, 1), the patcher should either clamp the value or issue a warning.  
   * *Fixedness*: bbr allows users to decide whether to inherit the FIXED status.10 If the parent parameter was fixed to 0, should the child also fix it? The UI must present this option.  
   * **$OMEGA/$SIGMA**: These are matrices. The patcher must determine if the block is defined as DIAGONAL (vector of variances) or BLOCK (full covariance matrix). The parent estimates must be mapped correctly to the structural definition in the child file.  
6. **Write File**: Save the modified control stream to disk.

This logic represents the core "business logic" that must be ported from R (bbr/nmrec) to Go.

## ---

**6\. Integration Strategy: The "Go-MERGE" Fyne Application**

Integrating these features into a coherent Fyne application requires a clean architectural pattern, likely Model-View-Controller (MVC).

### **6.1 Application Structure**

1. **The Model (Data Layer)**:  
   * **Structs**: Run, Parameter, TableData.  
   * **Logic**: Import metrumresearchgroup/bbi/parsers for reading files. Implement the custom ControlStreamPatcher for inheritance.  
2. **The View (UI Layer)**:  
   * **Dashboard**: A widget.List displaying the models in the current directory. Each item shows the Run ID, status (parsed from bbi output), and tags.  
   * **Parameter Tab**: A widget.Table displaying the extracted parameters.  
   * **Plot Tab**: A container for canvas.Image objects holding the diagnostic plots generated by gonum.  
3. **The Controller (Workflow Layer)**:  
   * **Inheritance Action**: A button "New Run from Selected". This triggers the Copy logic, updates the YAML based\_on, and runs the ControlStreamPatcher.  
   * **Execution**: A wrapper around bbi/runner to spawn the NONMEM process when "Run" is clicked.

### **6.2 Implementation Roadmap**

To accomplish the user's goal, the development should follow this sequence:

1. **Phase 1: The BBI Wrapper**. Create a Go module that imports github.com/metrumresearchgroup/bbi. Verify it can successfully parse a local .lst file and return a RunDetails struct.  
2. **Phase 2: The Visualization Engine**. Implement the reader for .sdtab files and the gonum/plot logic to generate the DV-PRED scatter plot. Bind this to a Fyne window.  
3. **Phase 3: The Inheritance Logic**. Develop the standalone ControlStreamPatcher package. Test it against various NONMEM formats (e.g., estimates with and without bounds, fixed parameters).  
4. **Phase 4: The GUI Integration**. Wire the components together. Ensure that selecting a run in the dashboard triggers the parsing and plotting routines asynchronously to keep the UI responsive.

## ---

**7\. Detailed Analysis of Specific MERGE Components**

To provide the exhaustive detail required, we must look closer at the specific implementation details of the key components identified in the research.

### **7.1 Deep Dive: bbi (Go)**

The bbi repository is the most critical resource. It is organized as a standard Go CLI project.

* **cmd/bbi/main.go**: This is the entry point. It initializes the CLI application (likely using urfave/cli or cobra).18  
* **runner/**: This package manages the execution. It handles:  
  * Directory preparation (copying data files, cleaning up old run files).  
  * Process orchestration (calling nmfe or the grid submission script).  
  * Output collection.  
* **parsers/**: This is the "gold mine" for the user's request. It contains specific parsers for:  
  * **nmparser**: The primary parser for .lst files. It uses state-machine logic to scan through the file, identifying headers like FINAL PARAMETER ESTIMATE and MINIMUM VALUE OF OBJECTIVE FUNCTION.  
  * **ext**: A parser for the table-based .ext files. This is more robust than .lst parsing for extracting parameters because the format is strictly tabular (whitespace separated).  
  * **grd**: Parsing for gradient files (used in covariance steps).

**Integration Insight**: The Fyne application should use the ext parser for parameter values (as it provides higher precision) and the nmparser (listing) for metadata like "Run Time" and "Heuristic Errors" (e.g., covariance step abort).19

### **7.2 Deep Dive: bbr (R) and Inheritance Logic**

The bbr package contains the blueprint for the inheritance logic. Specifically, the model-management.R and param-estimates.R files in the bbr source code are the references for logic porting.20  
**The inherit\_param\_estimates Workflow in Detail**:

1. **Validation**: bbr first checks if the parent model actually finished successfully. Inheriting from a crashed run is dangerous. The Go app should check the bbi summary for termination codes.  
2. **Jittering**: bbr includes a feature called tweak\_initial\_estimates.10 This adds random noise (jitter) to the initial estimates. This is useful for sensitivity analysis (checking if the model converges to the same optimum from different starting points). A complete Go implementation should ideally include a "Jitter" option in the Inheritance dialog, multiplying the inherited values by $e^{\\text{noise}}$, where noise $\\sim N(0, \\sigma)$.  
3. **Provenance**: When a model inherits parameters, bbr adds a note or tag to the model log (e.g., "Initial estimates inherited from run 101"). The Go application should append to the notes field in the bbi.yaml to maintain this audit trail.10

### **7.3 Deep Dive: pmplots and Diagnostic logic**

The logic within pmplots is centered on effective communication of model fit.

* **Log-Scale Logic**: Pharmacokinetic data spans orders of magnitude. pmplots defaults to log-log scales for DV vs. PRED plots. The Fyne/Gonum implementation must support logarithmic axes.  
* **Stratification**: A key feature of pmplots is the ability to facet plots by covariates (e.g., Sex, Dose Group).5 In Fyne, this could be implemented as a "Group By" dropdown that filters the data passed to the plotter or generates a grid of sub-plots.  
* **Residual Analysis**: pmplots calculates CWRES (Conditional Weighted Residuals) if they are not present, but modern NONMEM versions output CWRESI (with Interaction) directly. The Go app should prioritize reading CWRESI or CWRES columns from the table files.

## ---

**8\. Conclusion and Strategic Recommendations**

The Metrum Research Group Ecosystem (MERGE) provides a rigorous, validated, and comprehensive framework for pharmacometrics. However, its heavy reliance on R for the "human-in-the-loop" aspects (workflow, visualization, model modification) presents a significant barrier to a pure Golang implementation.  
To successfully build the requested Fyne.io application, the developer cannot simply "wrap" the existing tools. They must:

1. **Adopt bbi**: Use the metrumresearchgroup/bbi Go module as the foundational engine for execution and output parsing.  
2. **Port the Logic**: Re-implement the control stream manipulation logic from nmrec and the inheritance workflow from bbr into native Go code.  
3. **Rebuild Visualization**: Use gonum/plot to recreate the standard diagnostic plots defined by pmplots.

This architectural approach respects the design principles of MERGE—reproducibility, traceability, and separation of concerns—while achieving the performance and deployment benefits of a standalone Golang application. By strictly adhering to the bbi JSON schemas and the bbr YAML configurations, the new tool can remain compatible with the existing MERGE ecosystem, allowing users to switch between the R-based workflow and the new Go-based GUI without friction.

#### **Works cited**

1. MeRGE Expo \- Metrum Research Group, accessed December 8, 2025, [https://www.metrumrg.com/merge-expo/](https://www.metrumrg.com/merge-expo/)  
2. Methodology, Tools, and Computation Archives \- Metrum Research Group, accessed December 8, 2025, [https://metrumrg.com/publications/methodology-tools-and-computation/](https://metrumrg.com/publications/methodology-tools-and-computation/)  
3. metrumresearchgroup/bbi: Next generation modeling platform \- GitHub, accessed December 8, 2025, [https://github.com/metrumresearchgroup/bbi](https://github.com/metrumresearchgroup/bbi)  
4. metrumresearchgroup/bbr: R interface for model and project management \- GitHub, accessed December 8, 2025, [https://github.com/metrumresearchgroup/bbr](https://github.com/metrumresearchgroup/bbr)  
5. Plots for Pharmacometrics • pmplots \- Metrum Research Group, accessed December 8, 2025, [https://metrumresearchgroup.github.io/pmplots/](https://metrumresearchgroup.github.io/pmplots/)  
6. metrumresearchgroup/nmrec: R package to read, parse, and modify NONMEM control records \- GitHub, accessed December 8, 2025, [https://github.com/metrumresearchgroup/nmrec](https://github.com/metrumresearchgroup/nmrec)  
7. A completely open-source pharmacometrics tool set: Moving from vision to reality with R, mrgsolve and Stan/Torsten Marc Gastongu, accessed December 8, 2025, [https://www.metrumrg.com/wp-content/uploads/2019/11/ACoP10-Tutorial-3-MetrumRG.pdf](https://www.metrumrg.com/wp-content/uploads/2019/11/ACoP10-Tutorial-3-MetrumRG.pdf)  
8. Metrum Research Group \- GitHub, accessed December 8, 2025, [https://github.com/metrumresearchgroup](https://github.com/metrumresearchgroup)  
9. MeRGE Expo 1 \- About Metworx, accessed December 8, 2025, [https://merge.metrumrg.com/expo/expo1-nonmem-foce/posts/about-metworx.html](https://merge.metrumrg.com/expo/expo1-nonmem-foce/posts/about-metworx.html)  
10. Getting Started with bbr \- Metrum Research Group, accessed December 8, 2025, [https://metrumresearchgroup.github.io/bbr/articles/getting-started.html](https://metrumresearchgroup.github.io/bbr/articles/getting-started.html)  
11. Tools \- ISOP Website \- International Society of Pharmacometrics, accessed December 8, 2025, [https://www.isop.org/resources/tools](https://www.isop.org/resources/tools)  
12. pmtables: Tables for Pharmacometrics., accessed December 8, 2025, [https://mpn.metworx.com/packages/pmtables/0.3.1/reference/pmtables.html](https://mpn.metworx.com/packages/pmtables/0.3.1/reference/pmtables.html)  
13. Parses parameter estimates table — param\_estimates • bbr \- Metrum Research Group, accessed December 8, 2025, [https://metrumresearchgroup.github.io/bbr/reference/param\_estimates.html](https://metrumresearchgroup.github.io/bbr/reference/param_estimates.html)  
14. Use shrinkage file to get shrinkage data · Issue \#54 · metrumresearchgroup/bbi \- GitHub, accessed December 8, 2025, [https://github.com/metrumresearchgroup/babylon/issues/54](https://github.com/metrumresearchgroup/babylon/issues/54)  
15. MeRGE Expo 1 \- Creating a Parameter Key, accessed December 8, 2025, [https://merge.metrumrg.com/expo/expo1-nonmem-foce/posts/parameter-key.html](https://merge.metrumrg.com/expo/expo1-nonmem-foce/posts/parameter-key.html)  
16. bbr Parameter Labels \- Metrum Research Group, accessed December 8, 2025, [https://metrumresearchgroup.github.io/bbr/articles/parameter-labels.html](https://metrumresearchgroup.github.io/bbr/articles/parameter-labels.html)  
17. Package index • pmplots \- Metrum Research Group, accessed December 8, 2025, [https://metrumresearchgroup.github.io/pmplots/reference/index.html](https://metrumresearchgroup.github.io/pmplots/reference/index.html)  
18. Build \#1045.2.2 \- metrumresearchgroup/bbi \- Drone CI, accessed December 8, 2025, [https://github-drone.metrumrg.com/metrumresearchgroup/bbi/1045/2/2](https://github-drone.metrumrg.com/metrumresearchgroup/bbi/1045/2/2)  
19. Summarize model outputs — model\_summary • bbr, accessed December 8, 2025, [https://mpn.metworx.com/packages/bbr/1.1.3/reference/model\_summary.html](https://mpn.metworx.com/packages/bbr/1.1.3/reference/model_summary.html)  
20. Package index • bbr \- Metrum Research Group, accessed December 8, 2025, [https://metrumresearchgroup.github.io/bbr/reference/index.html](https://metrumresearchgroup.github.io/bbr/reference/index.html)