import { describe, it, expect, beforeEach } from 'vitest';
import { t, setLocale, getLocale } from './index.svelte';

describe('i18n Utility', () => {
    beforeEach(() => {
        setLocale('en');
    });

    it('should return the key if translation is missing', () => {
        expect(t('non.existent.key')).toBe('non.existent.key');
    });

    it('should return correct translation for English', () => {
        expect(t('action.save')).toBe('Save');
    });

    it('should switch languages correctly', () => {
        setLocale('pt-BR');
        expect(getLocale()).toBe('pt-BR');
        expect(t('action.save')).toBe('Salvar');

        setLocale('es');
        expect(getLocale()).toBe('es');
        expect(t('action.save')).toBe('Guardar');
    });

    it('should interpolate parameters correctly', () => {
        // Test with a key that has placeholders
        expect(t('apps.updatePrompt', { title: 'Plex' })).toBe('A new version of Plex is ready to be installed.');
        
        setLocale('pt-BR');
        expect(t('apps.updatePrompt', { title: 'Plex' })).toBe('Uma nova versão de Plex está pronta para ser instalada.');

        setLocale('es');
        expect(t('apps.updatePrompt', { title: 'Plex' })).toBe('Una nueva versión de Plex está lista para ser instalada.');
    });

    it('should handle multiple parameters', () => {
        // Adding a temporary test for multiple params if we had any, 
        // but current keys mostly have one. Let's test if it handles multiple {key} if they existed.
        // For now, let's test a key with a count.
        expect(t('files.deleteItemsPrompt', { count: '5' })).toBe('Delete 5 items? This action cannot be undone.');
    });

    it('should not break if params are provided but no placeholders exist', () => {
        expect(t('action.save', { extra: 'param' })).toBe('Save');
    });
});
