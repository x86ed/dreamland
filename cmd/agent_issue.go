package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"dreamland/internal/agentissue"
	"dreamland/internal/config"
	"dreamland/internal/scaffold"
)

var agentIssueCmd = &cobra.Command{
	Use:   "agent-issue",
	Short: "Write the new-agent issue form or create a new-agent GitHub issue",
	Long: "--template writes (or refreshes) .github/ISSUE_TEMPLATE/new-agent.yml.\n" +
		"--create files a new-agent issue through gh after showing it and asking for confirmation " +
		"(--yes skips the prompt; agents must not pass it). Exit codes are deterministic: 0 on success, 1 otherwise, " +
		"so it is safe to call from a lifecycle hook.",
	Args: cobra.NoArgs,
	RunE: runAgentIssue,
}

func init() {
	rootCmd.AddCommand(agentIssueCmd)
	f := agentIssueCmd.Flags()
	f.Bool("template", false, "write or refresh .github/ISSUE_TEMPLATE/new-agent.yml")
	f.Bool("create", false, "create a new-agent issue via gh")
	f.Bool("yes", false, "skip the confirmation prompt (human-driven scripting only)")
	f.String("name", "", "agent name")
	f.String("role", "", "one-line role")
	f.String("rationale", "", "rationale and evidence")
	f.String("tier", "", "tool tier: "+strings.Join(agentissue.Tiers, ", "))
	f.String("routing", "", "routing position (receives from / hands off to)")
	f.String("criteria", "", "acceptance criteria")
}

func runAgentIssue(cmd *cobra.Command, _ []string) error {
	fl := cmd.Flags()
	tmpl, _ := fl.GetBool("template")
	create, _ := fl.GetBool("create")
	if !tmpl && !create {
		return errors.New("specify --template or --create")
	}
	out := cmd.OutOrStdout()

	if create {
		s := func(n string) string { v, _ := fl.GetString(n); return v }
		f := agentissue.Fields{Name: s("name"), Role: s("role"), Rationale: s("rationale"), Tier: s("tier"), Routing: s("routing"), Criteria: s("criteria")}
		if err := f.Validate(); err != nil {
			return err
		}
		fmt.Fprintln(out, f.Preview())
		if yes, _ := fl.GetBool("yes"); !yes {
			fmt.Fprint(out, "Create this issue? [y/N] ")
			line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if strings.TrimSpace(line) != "y" {
				return errors.New("aborted: issue not created")
			}
		}
		url, err := agentissue.Create(f)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, url)
	}

	if tmpl {
		cwd, err := osGetwd()
		if err != nil {
			return err
		}
		root, err := config.FindRepoRoot(cwd)
		if err != nil {
			return err
		}
		r, err := scaffold.InstallIssueTemplate(root, true)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s: %s\n", r.Action, r.Path)
	}
	return nil
}
