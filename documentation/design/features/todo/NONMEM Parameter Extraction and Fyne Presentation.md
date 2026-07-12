

# **Automated Pharmacometric Workflow: Go Implementation for NONMEM Parameter Extraction and Fyne.io Visualization**

## **I. The Pharmacometric Imperative: Rationale for Sequential Estimation**

The execution of Nonlinear Mixed Effects (NLME) models using software such as NONMEM is fundamentally an iterative process, aimed at maximizing the joint likelihood of observed data. Success hinges on finding the optimal set of population fixed effects (THETA), inter-individual variability parameters (OMEGA), and residual variability parameters (SIGMA).1 THETA represents the typical values of pharmacokinetic (PK) or pharmacodynamic (PD) parameters for the population, OMEGA describes the covariance matrix of the inter-individual random effects (ETAs), and SIGMA describes the covariance matrix of the residual random effects (EPSILONs).3 The primary challenge in this optimization task is numerical stability and computational efficiency.4

### **1.1. Contextualizing Population PK/PD Modeling and Parameter Roles**

NLME model building typically progresses through a series of increasingly complex models, starting with a base structural model, followed by the addition of variability components and finally covariate exploration.5 The computational goal is to rapidly and reliably minimize the Objective Function Value (OFV) to find the set of parameters that maximize the likelihood.4

The minimization process relies on iterative gradient searches (or expectation-maximization methods like SAEM or IMP) across a complex, often non-convex, objective function landscape.1 A critical factor is that the effectiveness and reliability of these iterative optimization algorithms depend heavily on their starting point, known as the Initial Estimates (IEs).7 A complex likelihood surface, particularly common with advanced models or sparse data, often contains multiple *local minima*.8 If the optimization algorithm begins far from the true global minimum (i.e., using arbitrary IEs), it is highly likely to converge prematurely to a local minimum or fail to converge altogether, resulting in biased, incorrect, or unreliable parameter estimates.7

Therefore, the historical log requested by the user serves as an essential Quality Control (QC) measure, allowing modelers to track the stability of the model against this inherent numerical instability.

### **1.2. The Criticality of Initial Estimates (IEs) and Numerical Stability**

The selection of IEs is a strategic modeling decision that directly influences the computational burden and the reliability of the outcome.7 A well-chosen set of IEs influences the subsequent Objective Function Value (OFV) minimization process, ensuring that the gradient descent steps are directed toward the global optimum.7 The guidance for setting IEs suggests starting with relatively small values for OMEGA, as precise guesses are difficult, while recommending sufficiently large values for residual error parameters (SIGMA) to prevent convergence failure.7 The consequences of poor IEs are severe, ranging from outright convergence failure to termination with suboptimal parameter estimates.7

This dependence on IEs transforms into a measure of optimization efficiency. The closer the IEs are to the final converged solution, the fewer numerical iterations are required to satisfy convergence criteria (such as NSIG, SIGL, or TOL settings).1 This proximity to the optimal solution directly translates to a significant reduction in computation time. For instance, in simulation studies, using the true underlying parameter values as IEs has been shown to result in near 100% convergence rates 10, demonstrating the maximal theoretical benefit of accurate starting points. Similarly, studies have shown that providing better IEs can reduce computational run time for high-cost methods (like FOCEI, ITS, or IMP) by factors of three to ten.6 Monitoring run time alongside parameter changes in the historical log provides a quantitative metric for assessing optimization efficiency throughout the development sequence.

### **1.3. Sequential Optimization Strategy (The Pirana/PsN Paradigm)**

The practice of sequential initialization—using the final estimates (FEST) from a successfully converged model (Run N) as the initial estimates (IEs) for the next model (Run N+1)—is central to all modern iterative population modeling workflows. This strategy is precisely what tools like Pirana and PsN automate via the update\_inits script.11

The approach ensures numerical continuity and stability, whether progressing through structural model building, incorporating covariates 5, or chaining different estimation methods. A classic example of method chaining is using the Stochastic Approximation Expectation-Maximization (SAEM) method, which is fast and robust for preliminary estimates, followed by the Importance Sampling (IMP) method to calculate the marginal likelihood objective function value.1 In this sequence, the final, robust SAEM estimates are explicitly used as the precise starting points for the subsequent IMP step, vastly improving IMP's efficiency and accuracy.1

Furthermore, tracking the historical evolution of parameters provides a critical diagnostic tool. If an iterative step (Run N $\\rightarrow$ Run N+1) converges successfully but exhibits a large, unexpected change, or "drift," in core parameters (THETA, OMEGA, SIGMA), it signals potential identifiability issues or instability in the underlying model structure.13 By systematically logging this historical context, the pharmacometrician can programmatically identify and flag runs that require rigorous diagnostic review (e.g., sensitivity analysis or re-examination of the control stream setup). The historical log thus functions as an essential, auditable quality control mechanism.

The following table summarizes the quantitative and qualitative benefits of implementing sequential parameter initialization:

Table: Pharmacometric Benefits of Sequential Parameter Initialization (FEST to IE)

| Benefit | Impact | Quantitative Mechanism | Evidence & Context |
| :---- | :---- | :---- | :---- |
| Improved Convergence Success Rate | Avoids local minima and non-convergence errors. | Starts closer to the true Maximum Likelihood, reducing dependence on initial parameter perturbation.8 | Increases stability, particularly in complex models or when moving from SAEM to IMP steps.1 |
| Reduced Computational Time | Faster optimization and turnaround time for iterative modeling. | Fewer iterations needed for the gradient search to satisfy convergence criteria (NSIG, SIGL, TOL).1 | Can reduce runtime by 3-10 times for computationally intensive methods (e.g., FOCEI, ITS, IMP).6 |
| Enhanced Model Robustness | Consistency of estimates across model updates (e.g., adding a covariate). | Provides a physiologically plausible and numerically tested starting configuration for THETA, OMEGA, and SIGMA.7 | Essential for automated systems like SCM, where thousands of sequential runs occur.16 |

## **II. Architecture of the Historical Run Log Data Model (Go Lang)**

To build a robust historical run log in Go, a strong, well-defined data architecture is required, encompassing both the selection of the data source and the design of the persistence layer.

### **2.1. Selection and Structure of the NONMEM Output Source**

While the NONMEM report file (.lst or .res) contains the final estimates in a human-readable format, along with diagnostics like standard errors and shrinkage estimates 17, the optimal source for programmatic extraction is the raw output extension file (.ext). The .ext file is superior because it provides a consistent, machine-readable format containing all estimation iterations, not just the final result.18

The .ext file structure is semi-structured, accommodating data generated from multiple estimation steps within a single control stream. Each estimation step ($EST) generates a header line beginning with TABLE NO. n, where $n$ increments sequentially.20 If the model employs multiple estimation methods (e.g., SAEM followed by IMP), the .ext file will contain multiple such tables.20 The final converged estimate (FEST) is invariably located in the last table generated, corresponding to the highest value of $n$, or the last iteration within that final table.22

The inherent structure of the .ext file is critical for parsing: the data columns explicitly identify the parameter type (THETA, OMEGA, SIGMA), the parameter name, and, for matrix elements, their positional indices (i and j columns).21 This indexing information is essential for accurate reconstruction of the covariance matrices. Although the user requires only the final estimates, the Go parser must necessarily process the iterative data structure to efficiently locate the *last* iteration within the *last* table, mirroring the index="last" logic employed in high-level pharmacometric tools.22 This ensures low-latency retrieval, even when dealing with large output files generated by high-volume Bayesian MCMC simulations.14

### **2.2. Defining the Core Data Model (Go Structs)**

For persistence and efficient retrieval, the historical log data must be captured in a strongly typed Go structure. The use of structured data formats like JSON, often facilitated by Go logging packages such as logrus or Zap, is highly recommended for scientific logs, ensuring the data is easily queryable and archived.23 The design must account for the variable nature of the parameters while ensuring that the matrix data remains accurately represented.

The following structure, NonmemRunResult, is recommended as the internal data model:

Table: Recommended Go Struct for Historical Run Data (NonmemRunResult)

| Field Name | Go Type | Description | Source File |
| :---- | :---- | :---- | :---- |
| RunID | string | Unique model run identifier (e.g., run001) | Context/Log |
| Timestamp | time.Time | Execution completion time for historical ordering | Context/Log |
| OFV | float64 | Final Minimum Objective Function Value | .ext |
| ConvergenceStatus | string | Termination message (e.g., successful, covariance omitted) | .lst (or parsed .ext status) |
| THETA | map\[string\]float64 | Fixed effect estimates, mapped by name (e.g., THETA1: 0.5) | .ext |
| THETA\_RSE | map\[string\]float64 | Relative Standard Error for THETA (for QC display) | .lst or .xml 17 |
| OMEGA\_Matrix | float64 | Full Inter-Individual Variability (IIV) Matrix | .ext |
| SIGMA\_Matrix | float64 | Full Residual Variability (RV) Matrix | .ext |

The structure leverages maps for the arbitrary number of THETA parameters and two-dimensional slices (float64) to preserve the mathematical integrity of the OMEGA and SIGMA variance-covariance matrices.27

This internal representation, while accurate, contrasts sharply with the requirements of the Fyne.io graphical interface. The Fyne table necessitates a flat, row-by-row representation for efficient visualization. Therefore, the architecture requires an intermediate Presentation Data Layer (PDL) that transforms the complex internal model (e.g., the OMEGA\_Matrix) into a simplified, display-ready format. This transformation involves flattening key matrix components, such as extracting only the diagonal elements (the variances of IIV and RV parameters), to create a single row summarizing the run's essential characteristics. This layered approach ensures that the high performance of the Fyne UI is maintained while the core scientific data remains strongly typed and structurally sound. For historical access, this structured data should be managed as a persistent data structure, ensuring that prior run versions (historical states) are maintained efficiently as the model library grows.28

## **III. Golang Implementation for Parameter Extraction and Parsing**

The extraction process must handle the unique formatting challenges presented by scientific computation output files.

### **3.1. Handling Scientific Text and Fixed-Width Formatting**

NONMEM output, particularly the iterative estimation data within the .ext file, relies heavily on consistent spacing for column definition, resulting in fixed-width fields, often containing numerical data in scientific notation.21 Relying on simple, naive string splitting (e.g., strings.Split) based on whitespace is highly unreliable due to the variable precision and scientific notation of the floating-point numbers.

A more robust solution involves utilizing a specialized library designed for structured parsing. The go-fixedwidth package is purpose-built for this exact challenge, providing encoding and decoding capabilities for fixed-width data.29 By defining the expected character offsets and widths within Go struct tags, the parser can accurately ingest numerical values, even those presented in scientific notation, minimizing the fragility associated with manual text processing.

Alternatively, a lower-level approach can use bufio.NewReader combined with fmt.Fscanf to define the expected numerical formats (%f, %e) for ingestion.32 However, this method is still highly sensitive to potential variations in output spacing across different NONMEM versions, making the dedicated fixed-width library a more resilient choice for a production pharmacometric tool.

### **3.2. Extraction Logic for Final Estimates (FEST)**

The core logic of the Go parser must operate as a robust state machine to navigate the structure of the .ext file.20

The process begins by scanning the file line-by-line, transitioning through states:

1. **Search for Table Header:** Locate the latest instance of the TABLE NO. n header, ensuring the parser targets the final estimation step if multiple $EST records exist.20  
2. **Parse Column Definition:** Read the subsequent line(s) to identify column names, which typically include generic labels like THETA1, OMEGA1, and diagnostics such as OFV, i, and j.21  
3. **Parse Iterations:** Begin parsing the data rows. Since the final estimate is the last iteration, the parser must read all rows, storing the parameters, Objective Function Value (OFV), and termination status. The last set of parameters read will represent the Final Estimates of the run.18

#### **Reconstructing the Covariance Matrices**

A critical technical complexity involves reconstructing the OMEGA and SIGMA matrices. The .ext file flattens these matrices, listing elements (variances and covariances) sequentially, with corresponding i and j indices to indicate their matrix position.21 The Go parser must implement dynamic reconstruction logic: reading the value, identifying its type (OMEGA or SIGMA), and placing it at the precise \[i\]\[j\] location within the internal float64 matrix structure.21

This reconstruction ensures the mathematical integrity of the variance-covariance matrices. Since OMEGA and SIGMA are primary initial estimates (IEs) for the subsequent sequential model run, any failure to correctly map $i$ and $j$ coordinates would lead to corrupted IEs, undermining the core pharmacometric strategy of sequential initialization. Once the matrices and THETA map are fully constructed, they are serialized and appended to the historical log for long-term storage and immediate use in updating the control file for the next model run—the software analog of PsN's update\_inits functionality.12

## **IV. High-Performance Visualization using Fyne.io**

The visualization layer must efficiently display a potentially large and growing historical record of model runs without compromising application performance. Fyne.io's design, specifically its collection widgets, provides the necessary architectural foundation.

### **4.1. The Fyne.io Table Widget and Callback Architecture**

The widget.Table is the appropriate component for displaying the run history, as it provides a performant two-dimensional index structure analogous to the List widget.33 Crucially, the Table widget avoids the performance drain associated with rendering large datasets by utilizing a callback architecture rather than embedding all data into memory simultaneously.33

Three main callback functions must be implemented:

1. **Length:** A function returning two integers: the total number of historical runs (rows) and the number of desired summary parameters (columns).33  
2. **CreateCell:** A function that returns a template fyne.CanvasObject (e.g., widget.NewLabel) used for rendering cells. This allows the framework to cache template objects efficiently.33  
3. **UpdateCell:** The core rendering function that applies the specific data to the cell template based on the (row, col) index provided. This ensures that only visible cells are actively populated with data.33

This callback mechanism mandates a strict decoupling between the internal data model (NonmemRunResult struct) and the presentation layer. A Presentation Data Layer (PDL) is required to dynamically retrieve the correct NonmemRunResult based on the row index, flatten the complex numerical data (maps, matrices) into formatted strings (e.g., rounding float64 values to a user-defined precision), and return that string to the UpdateCell function for display. This decoupling maintains application responsiveness regardless of the run log's size.

### **4.2. Designing the Historical Parameter History View**

The design of the visualization must prioritize the immediate identification of convergence status and parameter evolution, enabling quick quality assessment by the pharmacometrician. The Fyne widget.NewTableWithHeaders function should be employed to provide clear context for the dozens of parameters and metrics displayed.34

The utility of the historical log transcends merely listing numbers; it must actively diagnose the model building progression. Therefore, the implementation should incorporate visual metrics directly into the table rendering via the UpdateCell callback:

1. **Status Indication:** A dedicated Status column should display the ConvergenceStatus (e.g., "Successful Termination" vs. "Covariance Step Omitted"). Using color coding (e.g., a green indicator for successful convergence and red for failure) provides an immediate diagnostic flag.10  
2. **Parameter Drift Quantification:** A critical feature for quality control is tracking how much a key parameter changed from one sequential run to the next. Columns should be included that display the percentage change ($\\Delta\\%$) for crucial fixed effects (THETA) and variability components (e.g., the standard deviation, $\\sqrt{\\text{OMEGA}\_{i,i}}$). This requires the PDL to calculate $P\_{N} \- P\_{N-1} / P\_{N-1}$ on demand.  
3. **Visual Flagging:** Employing color coding based on the magnitude of the $\\Delta\\%$ (e.g., slight green/white for minimal, expected changes; yellow or red for large, suspicious shifts exceeding a predefined threshold) acts as an immediate QC flag, visually isolating runs where potential instability or unexpected identifiability issues occurred.13

The proposed table structure below outlines the necessary columns for effective model monitoring:

Table: Proposed Fyne.io Historical Log Structure

| Column Header | Source Data | Fyne.io Widget Type | Visualization Role |
| :---- | :---- | :---- | :---- |
| **Run ID** | RunID | widget.NewLabel | Context and link to raw files. |
| **OFV** | OFV | widget.NewLabel | Primary fitness metric; track minimization path. |
| **THETA (CL)** | THETA\["CL"\] | widget.NewLabel | Tracking fixed effect value.7 |
| **OMEGA (CL) SD** | sqrt(OMEGA\_Matrix\[i\]\[i\]) | widget.NewLabel | Tracking inter-individual variability (IIV) magnitude.3 |
| **$\\Delta\\%$ (THETA CL)** | Calculated difference from Run N-1 | widget.NewLabel (Color coded) | **Critical QC:** Highlights parameter drift or rapid shifts, flagging potential instability. |
| **Status** | ConvergenceStatus | widget.NewLabel (Color coded) | Immediate visualization of estimation success.10 |

## **V. Synthesis and Comprehensive Implementation Plan**

The development of this tool component requires the coordinated integration of pharmacometric requirements, Go programming techniques, and high-performance UI design.

### **5.1. The Complete Go-Fyne.io Pipeline Architecture**

The overall pipeline operates as a closed loop, where the results of one model execution dictate the starting conditions of the next, while simultaneously documenting the historical progression.

1. **Execution Monitoring:** The custom NONMEM executor tool submits the model and, upon completion, monitors the output directory for the .ext file (and .lst or .xml for additional diagnostics like RSEs).17  
2. **Data Ingestion (Go Parsing):** A dedicated Go module uses the state machine logic and the go-fixedwidth library to parse the raw .ext data. It identifies the final iteration, reconstructs the THETA map, OMEGA, and SIGMA matrices, and extracts the final OFV and convergence status.20  
3. **Data Modeling and Persistence:** The parsed data is converted into the structured NonmemRunResult Go struct and immediately serialized (e.g., to JSON using a structured logger) into the persistent historical run log file. This ensures that the run log is an append-only, auditable record.24  
4. **Sequential Preparation (The update\_inits Analog):** This crucial step closes the loop. A Go function reads the final numerical estimates (THETA, OMEGA, SIGMA) from the newly persisted NonmemRunResult and automatically updates the $THETA, $OMEGA, and $SIGMA records within the input control file for the subsequent run (Run N+1).11 This implementation replicates the core function of PsN's update\_inits, ensuring that the pharmacometric benefits of increased convergence success and reduced runtime are realized (Section I).  
5. **Visualization (Fyne.io):** The Fyne application reads the serialized historical log data, manages it in the PDL, and presents it efficiently using the widget.Table structure and its performance-optimized callback functions.

### **5.2. Technical Deep Dive: Go Parsing Library Selection**

The success of the data ingestion step hinges on reliably parsing the fixed-width, scientific output of the .ext file.

While customizing Go’s standard libraries using fmt.Fscanf or regular expressions is possible, it creates fragile code highly susceptible to minor changes in NONMEM’s output formatting (e.g., changes in significant digits or spacing).32 The use of the go-fixedwidth library is the most robust technical solution.29 This library specifically addresses the fixed-width parsing challenge by allowing developers to define column positions directly via struct tags, automatically handling the conversion of scientific strings into Go's native numerical types (float64). Assuming the .ext file format maintains consistent column widths across iterations and subsequent estimation steps, go-fixedwidth dramatically simplifies the parsing complexity and enhances the long-term maintainability of the Go component.

### **5.3. Strategic Implication: The Role of the Historical Log in QC/QA**

The integration of parameter extraction and historical display is not merely an automation convenience; it establishes a rigorous framework for Quality Control (QC) and auditable traceability—a non-negotiable requirement for regulatory pharmacometric submissions.

In regulatory contexts, the entire model development sequence must be justified and transparent.5 By programmatically enforcing sequential initialization and systematically logging the extracted final estimates and convergence status for every run, the user establishes a clear, auditable trace that supports the final model selection. If the historical log demonstrates that core parameters (THETA, OMEGA, SIGMA) remain stable and evolve predictably across multiple iterative steps (e.g., during covariate addition or removal), it provides strong evidence supporting the final model's structural robustness and fitness. Conversely, the immediate visual flagging of significant parameter drift ($\\Delta\\%$) ensures that potential instability or unreliable optimization steps are identified early in the process, directing the modeler to re-evaluate or perform supplementary sensitivity analyses.13

## **Conclusions and Recommendations**

The objective of integrating NONMEM parameter extraction, sequential initialization, and historical visualization within a Go/Fyne.io pipeline is validated by the fundamental numerical challenges inherent in NLME modeling.

1. **Mandatory Pharmacometric Practice:** Sequential initialization is crucial because it transforms a highly complex, unstable optimization problem into a manageable iterative sequence. By using the Final Estimates (FEST) of Run N as the Initial Estimates (IEs) for Run N+1, the system achieves substantially higher convergence success rates and significantly reduced computational time compared to starting each run with arbitrary IEs.  
2. **Recommended Data Architecture:** The Go component must target the NONMEM .ext file for parameter extraction due to its comprehensive, machine-readable structure containing all iterative results and index data. The data should be stored in a strongly typed Go struct (NonmemRunResult) that leverages map\[string\]float64 for THETA and float64 for OMEGA/SIGMA to preserve scientific integrity.  
3. **Technical Implementation:** Robust parsing of the scientific fixed-width output is best achieved using the specialized go-fixedwidth package, which simplifies the extraction of the final estimates and the critical task of reconstructing the variance-covariance matrices from indexed data.  
4. **Visualization Strategy:** The Fyne.io application should utilize the widget.Table and implement a Presentation Data Layer (PDL) to efficiently transform the structured Go data into a flat, string-based format for high-performance display. This visualization is essential for QC, recommending the inclusion of convergence status indicators and calculated parameter drift ($\\Delta\\%$) columns to immediately flag runs that require deeper diagnostic review.

Successful implementation of this integrated pipeline provides the user with an analog to commercial tools like Pirana, converting parameter tracking from a manual diagnostic chore into an automated, auditable, and performance-enhancing element of the pharmacometric workflow.

#### **Works cited**

1. NONMEM Tutorial Part II: Estimation Methods and Advanced Examples \- PMC, accessed November 9, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC6709422/](https://pmc.ncbi.nlm.nih.gov/articles/PMC6709422/)  
2. NONMEM | Nonlinear Mixed Effects Modelling \- ICON plc, accessed November 9, 2025, [https://www.iconplc.com/solutions/technologies/nonmem](https://www.iconplc.com/solutions/technologies/nonmem)  
3. Development of a population PK model using NONMEM® \- Case Study I, accessed November 9, 2025, [https://www.pharmacy.umaryland.edu/media/SOP/wwwpharmacyumarylandedu/centers/ctm/CSINONMEM.pdf](https://www.pharmacy.umaryland.edu/media/SOP/wwwpharmacyumarylandedu/centers/ctm/CSINONMEM.pdf)  
4. (PDF) Handling Missing Dosing History in Population Pharmacokinetic Modeling: An Extension to MDM Method \- ResearchGate, accessed November 9, 2025, [https://www.researchgate.net/publication/329577201\_Handling\_Missing\_Dosing\_History\_in\_Population\_Pharmacokinetic\_Modeling\_An\_Extension\_to\_MDM\_Method](https://www.researchgate.net/publication/329577201_Handling_Missing_Dosing_History_in_Population_Pharmacokinetic_Modeling_An_Extension_to_MDM_Method)  
5. Covariate selection in pharmacometric analyses: a review of methods \- PMC, accessed November 9, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC4294083/](https://pmc.ncbi.nlm.nih.gov/articles/PMC4294083/)  
6. (PDF) Comparison of Nonmem 7.2 estimation methods and parallel processing efficiency on a target-mediated drug disposition model \- ResearchGate, accessed November 9, 2025, [https://www.researchgate.net/publication/51815963\_Comparison\_of\_Nonmem\_72\_estimation\_methods\_and\_parallel\_processing\_efficiency\_on\_a\_target-mediated\_drug\_disposition\_model](https://www.researchgate.net/publication/51815963_Comparison_of_Nonmem_72_estimation_methods_and_parallel_processing_efficiency_on_a_target-mediated_drug_disposition_model)  
7. (PDF) Tips for the choice of initial estimates in NONMEM \- ResearchGate, accessed November 9, 2025, [https://www.researchgate.net/publication/308387723\_Tips\_for\_the\_choice\_of\_initial\_estimates\_in\_NONMEM](https://www.researchgate.net/publication/308387723_Tips_for_the_choice_of_initial_estimates_in_NONMEM)  
8. Evaluation of bias, precision, robustness and runtime for estimation methods in NONMEM 7, accessed November 9, 2025, [https://www.researchgate.net/publication/262111075\_Evaluation\_of\_bias\_precision\_robustness\_and\_runtime\_for\_estimation\_methods\_in\_NONMEM\_7](https://www.researchgate.net/publication/262111075_Evaluation_of_bias_precision_robustness_and_runtime_for_estimation_methods_in_NONMEM_7)  
9. Tips for the choice of initial estimates in NONMEM \- KoreaMed Synapse, accessed November 9, 2025, [https://synapse.koreamed.org/articles/1082620](https://synapse.koreamed.org/articles/1082620)  
10. Performance of Three Estimation Methods in Repeated Time-to-Event Modeling \- PMC \- NIH, accessed November 9, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC3032099/](https://pmc.ncbi.nlm.nih.gov/articles/PMC3032099/)  
11. PsN :: Download \- GitHub Pages, accessed November 9, 2025, [https://uupharmacometrics.github.io/PsN/download.html](https://uupharmacometrics.github.io/PsN/download.html)  
12. PsN :: Documentation \- GitHub Pages, accessed November 9, 2025, [https://uupharmacometrics.github.io/PsN/docs.html](https://uupharmacometrics.github.io/PsN/docs.html)  
13. Sensitivity analysis of a mixture model to determine genotype/phenotype \- PAGE Meeting, accessed November 9, 2025, [https://www.page-meeting.org/pdf\_assets/9595-page2007\_3.pdf](https://www.page-meeting.org/pdf_assets/9595-page2007_3.pdf)  
14. Bayesian estimation in NONMEM \- PMC \- PubMed Central, accessed November 9, 2025, [https://pmc.ncbi.nlm.nih.gov/articles/PMC10864934/](https://pmc.ncbi.nlm.nih.gov/articles/PMC10864934/)  
15. a fast implementation for fitting pharmacometrics models to summary-level data in R \- bioRxiv, accessed November 9, 2025, [https://www.biorxiv.org/content/biorxiv/early/2025/10/29/2025.10.28.685047.full.pdf](https://www.biorxiv.org/content/biorxiv/early/2025/10/29/2025.10.28.685047.full.pdf)  
16. Nonmem via PsN \- Metworx, accessed November 9, 2025, [https://kb.metworx.com/Users/Tutorials/Nonmem/Nonmem-via-PsN/](https://kb.metworx.com/Users/Tutorials/Nonmem/Nonmem-via-PsN/)  
17. Creating a Nonmem Parameter Table, accessed November 9, 2025, [https://cran.r-project.org/web/packages/nonmemica/vignettes/parameter-table.html](https://cran.r-project.org/web/packages/nonmemica/vignettes/parameter-table.html)  
18. NMreadExt: Read information from Nonmem ext files in NMdata: Preparation, Checking and Post-Processing Data for PK/PD Modeling \- rdrr.io, accessed November 9, 2025, [https://rdrr.io/cran/NMdata/man/NMreadExt.html](https://rdrr.io/cran/NMdata/man/NMreadExt.html)  
19. NMautoverse/NMdata: Prepare and document data for Nonmem \- and automatically retrieve results \- GitHub, accessed November 9, 2025, [https://github.com/nmautoverse/NMdata/](https://github.com/nmautoverse/NMdata/)  
20. rawout.fil \- NONMEM Help, accessed November 9, 2025, [https://nmhelp.tingjieguo.com/rawout](https://nmhelp.tingjieguo.com/rawout)  
21. Read information from Nonmem ext files \- R, accessed November 9, 2025, [https://search.r-project.org/CRAN/refmans/NMdata/html/NMreadExt.html](https://search.r-project.org/CRAN/refmans/NMdata/html/NMreadExt.html)  
22. Extract estimates from NONMEM ext file — read\_nmext \- mrgsolve.org, accessed November 9, 2025, [https://mrgsolve.org/docs/reference/read\_nmext.html](https://mrgsolve.org/docs/reference/read_nmext.html)  
23. How to collect, standardize, and centralize Golang logs \- Datadog, accessed November 9, 2025, [https://www.datadoghq.com/blog/go-logging/](https://www.datadoghq.com/blog/go-logging/)  
24. Complete Guide to Logging in Go \- Golang Log | SigNoz, accessed November 9, 2025, [https://signoz.io/guides/golang-log/](https://signoz.io/guides/golang-log/)  
25. \[R\] Training LLMs for Strict JSON Schema Adherence via Reinforcement Learning and Structured Reasoning \- Reddit, accessed November 9, 2025, [https://www.reddit.com/r/MachineLearning/comments/1iwxtmb/r\_training\_llms\_for\_strict\_json\_schema\_adherence/](https://www.reddit.com/r/MachineLearning/comments/1iwxtmb/r_training_llms_for_strict_json_schema_adherence/)  
26. nonmem2R: Loading NONMEM Output Files with Functions for ..., accessed November 9, 2025, [https://cran.r-project.org/web/packages/nonmem2R/nonmem2R.pdf](https://cran.r-project.org/web/packages/nonmem2R/nonmem2R.pdf)  
27. Model library, accessed November 9, 2025, [https://cran.r-project.org/web/packages/pmxcode/vignettes/library.html](https://cran.r-project.org/web/packages/pmxcode/vignettes/library.html)  
28. Golang program to implement a persistent data structure (a stack) \- Tutorials Point, accessed November 9, 2025, [https://www.tutorialspoint.com/golang-program-to-implement-a-persistent-data-structure-a-stack](https://www.tutorialspoint.com/golang-program-to-implement-a-persistent-data-structure-a-stack)  
29. ianlopshire/go-fixedwidth: Encoding and decoding for fixed-width formatted data \- GitHub, accessed November 9, 2025, [https://github.com/ianlopshire/go-fixedwidth](https://github.com/ianlopshire/go-fixedwidth)  
30. fixedwidth package \- github.com/ianlopshire/go-fixedwidth \- Go Packages, accessed November 9, 2025, [https://pkg.go.dev/github.com/ianlopshire/go-fixedwidth](https://pkg.go.dev/github.com/ianlopshire/go-fixedwidth)  
31. o1egl/fwencoder: Fixed width file parser (encoder/decoder) in GO (golang) \- GitHub, accessed November 9, 2025, [https://github.com/o1egl/fwencoder](https://github.com/o1egl/fwencoder)  
32. Reading data from a text file with Go \- Stack Overflow, accessed November 9, 2025, [https://stackoverflow.com/questions/17125857/reading-data-from-a-text-file-with-go](https://stackoverflow.com/questions/17125857/reading-data-from-a-text-file-with-go)  
33. Table | Fyne Documentation, accessed November 9, 2025, [https://docs.fyne.io/collection/table/](https://docs.fyne.io/collection/table/)  
34. widget.Table | Fyne Documentation, accessed November 9, 2025, [https://docs.fyne.io/api/v2/widget/table/](https://docs.fyne.io/api/v2/widget/table/)